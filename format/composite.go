package format

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gobwas/glob"
	"github.com/numtide/treefmt/config"
	"github.com/numtide/treefmt/stats"
	"github.com/numtide/treefmt/walk"
	"golang.org/x/sync/errgroup"
	"mvdan.cc/sh/v3/expand"
)

const (
	batchKeySeparator = ":"
)

var ErrFormattingFailures = errors.New("formatting failures detected")

// batchKey represents the unique sequence of formatters to be applied to a batch of files.
// For example, "deadnix:statix:nixpkgs-fmt" indicates that deadnix should be applied first, statix second and
// nixpkgs-fmt third.
// Files are batched based on their formatting sequence, as determined by the priority and includes/excludes in the
// formatter configuration.
type batchKey string

// sequence returns the list of formatters, by name, to be applied to a batch of files.
func (b batchKey) sequence() []string {
	return strings.Split(string(b), batchKeySeparator)
}

func newBatchKey(formatters []*Formatter) batchKey {
	components := make([]string, 0, len(formatters))
	for _, f := range formatters {
		components = append(components, f.Name())
	}

	return batchKey(strings.Join(components, batchKeySeparator))
}

type batcher struct {
	batchSize  int
	batches    map[batchKey][]*walk.File
	formatters map[string]*Formatter

	signatures map[batchKey][]byte
}

func (b *batcher) formatSignature(key batchKey, matches []*Formatter) ([]byte, error) {
	signature, ok := b.signatures[key]
	if ok {
		return signature, nil
	}

	h := sha256.New()
	for _, f := range matches {
		if err := f.Hash(h); err != nil {
			return nil, fmt.Errorf("failed to hash formatter %s: %w", f.Name(), err)
		}
	}

	signature = h.Sum(nil)
	b.signatures[key] = signature

	return signature, nil
}

func (b *batcher) append(
	file *walk.File,
	matches []*Formatter,
) (appended bool, key batchKey, batch []*walk.File, err error) {
	slices.SortFunc(matches, formatterSortFunc)

	// construct a batch key based on the sequence of formatters
	key = newBatchKey(matches)

	// get format signature
	formatSignature, err := b.formatSignature(key, matches)
	if err != nil {
		return false, "", nil, err
	}

	//
	h := sha256.New()
	h.Write(formatSignature)
	h.Write([]byte(fmt.Sprintf("%v %v", file.Info.ModTime().Unix(), file.Info.Size())))

	file.FormatSignature = h.Sum(nil)

	if bytes.Equal(file.FormatSignature, file.CachedFormatSignature) {
		return false, key, b.batches[key], nil
	}

	// append to the batch
	b.batches[key] = append(b.batches[key], file)

	return true, key, b.batches[key], nil
}

func (b *batcher) reset(key batchKey) {
	b.batches[key] = make([]*walk.File, 0, b.batchSize)
}

func formatterSortFunc(a, b *Formatter) int {
	// sort by priority in ascending order
	priorityA := a.Priority()
	priorityB := b.Priority()

	result := priorityA - priorityB
	if result == 0 {
		// formatters with the same priority are sorted lexicographically to ensure a deterministic outcome
		result = cmp.Compare(a.Name(), b.Name())
	}

	return result
}

// CompositeFormatter handles the application of multiple Formatter instances based on global excludes and individual
// formatter configuration.
type CompositeFormatter struct {
	cfg            *config.Config
	stats          *stats.Stats
	batchSize      int
	globalExcludes []glob.Glob

	log            *log.Logger
	changeLevel    log.Level
	unmatchedLevel log.Level

	formatters map[string]*Formatter

	eg      *errgroup.Group
	batcher *batcher

	// formatError indicates if at least one formatting error occurred
	formatError *atomic.Bool
}

func (c *CompositeFormatter) apply(ctx context.Context, key batchKey, batch []*walk.File) {
	c.eg.Go(func() error {
		var formatErrors []error

		// apply the formatters in sequence
		for _, name := range key.sequence() {
			formatter := c.formatters[name]

			if err := formatter.Apply(ctx, batch); err != nil {
				formatErrors = append(formatErrors, err)
			}
		}

		// record if a format error occurred
		hasErrors := len(formatErrors) > 0
		c.formatError.Store(hasErrors)

		if !hasErrors {
			// record that the file was formatted
			c.stats.Add(stats.Formatted, len(batch))
		}

		// Create a release context.
		// We set no-cache based on whether any formatting errors occurred in this batch.
		// This is to communicate with any caching layer, if used when reading files for this batch, that it should not
		// update the state of any file in this batch, as we want to re-process them in later invocations.
		releaseCtx := walk.SetNoCache(ctx, hasErrors)

		// post-processing
		for _, file := range batch {
			// check if the file has changed
			changed, newInfo, err := file.Stat()
			if err != nil {
				return err
			}

			if changed {
				// record that a change in the underlying file occurred
				c.stats.Add(stats.Changed, 1)

				c.log.Log(
					c.changeLevel, "file has changed",
					"path", file.RelPath,
					"prev_size", file.Info.Size(),
					"prev_mod_time", file.Info.ModTime().Truncate(time.Second),
					"current_size", newInfo.Size(),
					"current_mod_time", newInfo.ModTime().Truncate(time.Second),
				)

				// update the file info
				file.Info = newInfo
			}

			// release the file as there is no further processing to be done on it
			if err := file.Release(releaseCtx); err != nil {
				return fmt.Errorf("failed to release file: %w", err)
			}
		}

		return nil
	})
}

// match filters the file against global excludes and returns a list of formatters that want to process the file.
func (c *CompositeFormatter) match(file *walk.File) []*Formatter {
	// first check if this file has been globally excluded
	if pathMatches(file.RelPath, c.globalExcludes) {
		c.log.Debugf("path matched global excludes: %s", file.RelPath)

		return nil
	}

	// a list of formatters that match this file
	var matches []*Formatter

	// otherwise, check if any formatters are interested in it
	for _, formatter := range c.formatters {
		if formatter.Wants(file) {
			matches = append(matches, formatter)
		}
	}

	return matches
}

// Apply applies the configured formatters to the given files.
func (c *CompositeFormatter) Apply(ctx context.Context, files []*walk.File) error {
	var toRelease []*walk.File

	for _, file := range files {
		matches := c.match(file) // match the file against the formatters

		// check if there were no matches
		if len(matches) == 0 {
			// log that there was no match, exiting with an error if the unmatched level was set to fatal
			if c.unmatchedLevel == log.FatalLevel {
				return fmt.Errorf("no formatter for path: %s", file.RelPath)
			}

			c.log.Logf(c.unmatchedLevel, "no formatter for path: %s", file.RelPath)

			// no further processing to be done, append to the release list
			toRelease = append(toRelease, file)

			// continue to the next file
			continue
		}

		// record there was a match
		c.stats.Add(stats.Matched, 1)

		appended, key, batch, batchErr := c.batcher.append(file, matches)
		if batchErr != nil {
			return fmt.Errorf("failed to append file to batch: %w", batchErr)
		}

		if len(batch) == c.batchSize {
			c.apply(ctx, key, batch)
			c.batcher.reset(key)
		}

		if !appended {
			toRelease = append(toRelease, file)
		}
	}

	// release files that require no further processing
	// we set noCache to true as there's no need to update the cache, since we skipped those files
	releaseCtx := walk.SetNoCache(ctx, true)

	for _, file := range toRelease {
		if err := file.Release(releaseCtx); err != nil {
			return fmt.Errorf("failed to release file: %w", err)
		}
	}

	return nil
}

// Close finalizes the processing of the CompositeFormatter, ensuring that any remaining batches are applied and
// all formatters have completed their tasks. It returns an error if any formatting failures were detected.
func (c *CompositeFormatter) Close(ctx context.Context) error {
	// flush any partial batches that remain
	// todo clean up
	for key, batch := range c.batcher.batches {
		if len(batch) > 0 {
			c.apply(ctx, key, batch)
		}
	}

	// wait for processing to complete
	if err := c.eg.Wait(); err != nil {
		return fmt.Errorf("failed to wait for formatters: %w", err)
	} else if c.formatError.Load() {
		return ErrFormattingFailures
	}

	return nil
}

// Hash takes anything that might affect the paths to be traversed, or how they are traversed, and adds it to a sha256
// hash.
// This can be used to determine if there has been a material change in config or setup that requires the cache
// to be invalidated.
func (c *CompositeFormatter) Hash() (string, error) {
	h := sha256.New()

	// sort formatters determinstically
	formatters := make([]*Formatter, 0, len(c.formatters))
	for _, f := range c.formatters {
		formatters = append(formatters, f)
	}

	slices.SortFunc(formatters, formatterSortFunc)

	// apply them to the hash
	for _, f := range formatters {
		if err := f.Hash(h); err != nil {
			return "", fmt.Errorf("failed to hash formatter: %w", err)
		}
	}

	// finalize
	return hex.EncodeToString(h.Sum(nil)), nil
}

func NewCompositeFormatter(
	cfg *config.Config,
	statz *stats.Stats,
	batchSize int,
) (*CompositeFormatter, error) {
	// compile global exclude globs
	globalExcludes, err := compileGlobs(cfg.Excludes)
	if err != nil {
		return nil, fmt.Errorf("failed to compile global excludes: %w", err)
	}

	// parse unmatched log level
	unmatchedLevel, err := log.ParseLevel(cfg.OnUnmatched)
	if err != nil {
		return nil, fmt.Errorf("invalid on-unmatched value: %w", err)
	}

	// create a composite formatter, adjusting the change logging based on --fail-on-change
	changeLevel := log.DebugLevel
	if cfg.FailOnChange {
		changeLevel = log.ErrorLevel
	}

	// create formatters
	formatters := make(map[string]*Formatter)

	env := expand.ListEnviron(os.Environ()...)

	for name, formatterCfg := range cfg.FormatterConfigs {
		formatter, err := newFormatter(name, cfg.TreeRoot, env, formatterCfg)

		if errors.Is(err, ErrCommandNotFound) && cfg.AllowMissingFormatter {
			log.Debugf("formatter command not found: %v", name)

			continue
		} else if err != nil {
			return nil, fmt.Errorf("failed to initialise formatter %v: %w", name, err)
		}

		// store formatter by name
		formatters[name] = formatter
	}

	//
	batcher := batcher{
		batchSize:  batchSize,
		formatters: formatters,
		batches:    make(map[batchKey][]*walk.File),
		signatures: make(map[batchKey][]byte),
	}

	// create an errgroup for asynchronously formatting
	eg := errgroup.Group{}
	// we use a simple heuristic to avoid too much contention by limiting the concurrency to runtime.NumCPU()
	eg.SetLimit(runtime.NumCPU())

	return &CompositeFormatter{
		cfg:            cfg,
		stats:          statz,
		batchSize:      batchSize,
		globalExcludes: globalExcludes,

		log:            log.WithPrefix("composite-formatter"),
		changeLevel:    changeLevel,
		unmatchedLevel: unmatchedLevel,

		eg:          &eg,
		batcher:     &batcher,
		formatters:  formatters,
		formatError: new(atomic.Bool),
	}, nil
}
