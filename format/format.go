package format

import (
	"cmp"
	"context"
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
	"github.com/numtide/treefmt/walk/cache"
	bolt "go.etcd.io/bbolt"
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

// batchMap maintains a mapping between batchKey and a slice of pointers to walk.File, used to organize files into
// batches based on the sequence of formatters to be applied.
type batchMap map[batchKey][]*walk.File

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

// Append adds a file to the batch corresponding to the given sequence of formatters and returns the updated batch.
func (b batchMap) Append(file *walk.File, matches []*Formatter) (key batchKey, batch []*walk.File) {
	slices.SortFunc(matches, formatterSortFunc)

	// construct a batch key based on the sequence of formatters
	key = newBatchKey(matches)

	// append to the batch
	b[key] = append(b[key], file)

	// return the batch
	return key, b[key]
}

// CompositeFormatter handles the application of multiple Formatter instances based on global excludes and individual
// formatter configuration.
type CompositeFormatter struct {
	stats          *stats.Stats
	batchSize      int
	globalExcludes []glob.Glob

	changeLevel    log.Level
	unmatchedLevel log.Level

	formatters map[string]*Formatter

	eg      *errgroup.Group
	batches batchMap

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

				log.Log(
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
		log.Debugf("path matched global excludes: %s", file.RelPath)

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

			log.Logf(c.unmatchedLevel, "no formatter for path: %s", file.RelPath)

			// no further processing to be done, append to the release list
			toRelease = append(toRelease, file)

			// continue to the next file
			continue
		}

		// record there was a match
		c.stats.Add(stats.Matched, 1)

		// check if the file is new or has changed when compared to the cache entry
		if file.Cache == nil || file.Cache.HasChanged(file.Info) {
			// add this file to a batch and if it's full, apply formatters to the batch
			if key, batch := c.batches.Append(file, matches); len(batch) == c.batchSize {
				c.apply(ctx, newBatchKey(matches), batch)
				// reset the batch
				c.batches[key] = make([]*walk.File, 0, c.batchSize)
			}
		} else {
			// no further processing to be done, append to the release list
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

// BustCache compares the currently configured formatters with their respective entries in the db.
// If a formatter was added, removed or modified, we clear any path entries from the cache, ensuring that all paths
// get formatted with the most recent formatter set.
func (c *CompositeFormatter) BustCache(db *bolt.DB) error {
	return db.Update(func(tx *bolt.Tx) error {
		clearPaths := false

		pathsBucket, err := cache.BucketPaths(tx)
		if err != nil {
			return fmt.Errorf("failed to get paths bucket from cache: %w", err)
		}

		formattersBucket, err := cache.BucketFormatters(tx)
		if err != nil {
			return fmt.Errorf("failed to get formatters bucket from cache: %w", err)
		}

		// check for any newly configured or modified formatters
		for name, formatter := range c.formatters {
			stat, err := os.Lstat(formatter.Executable())
			if err != nil {
				return fmt.Errorf("failed to stat formatter executable %v: %w", formatter.Executable(), err)
			}

			entry, err := formattersBucket.Get(name)
			if !(err == nil || errors.Is(err, cache.ErrKeyNotFound)) {
				return fmt.Errorf("failed to retrieve cache entry for formatter %v: %w", name, err)
			}

			isNew := errors.Is(err, cache.ErrKeyNotFound)
			hasChanged := !(isNew || (entry.Size == stat.Size() && entry.Modified == stat.ModTime()))

			if isNew {
				log.Debugf("formatter '%s' is new", name)
			} else if hasChanged {
				log.Debug("formatter '%s' has changed",
					name,
					"size", stat.Size(),
					"modTime", stat.ModTime(),
					"cachedSize", entry.Size,
					"cachedModTime", entry.Modified,
				)
			}

			// update overall flag
			clearPaths = clearPaths || isNew || hasChanged

			// record formatters info
			entry = &cache.Entry{
				Size:     stat.Size(),
				Modified: stat.ModTime(),
			}

			if err = formattersBucket.Put(name, entry); err != nil {
				return fmt.Errorf("failed to write cache entry for formatter %v: %w", name, err)
			}
		}

		// check for any removed formatters
		if err = formattersBucket.ForEach(func(key string, _ *cache.Entry) error {
			_, ok := c.formatters[key]
			if !ok {
				// remove the formatter entry from the cache
				if err = formattersBucket.Delete(key); err != nil {
					return fmt.Errorf("failed to remove cache entry for formatter %v: %w", key, err)
				}
				// indicate a clean is required
				clearPaths = true
			}

			return nil
		}); err != nil {
			return fmt.Errorf("failed to check cache for removed formatters: %w", err)
		}

		if clearPaths {
			// remove all path entries
			if err := pathsBucket.DeleteAll(); err != nil {
				return fmt.Errorf("failed to remove all path entries from cache: %w", err)
			}
		}

		return nil
	})
}

// Close finalizes the processing of the CompositeFormatter, ensuring that any remaining batches are applied and
// all formatters have completed their tasks. It returns an error if any formatting failures were detected.
func (c *CompositeFormatter) Close(ctx context.Context) error {
	// flush any partial batches that remain
	for key, batch := range c.batches {
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

	// create an errgroup for asynchronously formatting
	eg := errgroup.Group{}
	// we use a simple heuristic to avoid too much contention by limiting the concurrency to runtime.NumCPU()
	eg.SetLimit(runtime.NumCPU())

	return &CompositeFormatter{
		stats:          statz,
		batchSize:      batchSize,
		globalExcludes: globalExcludes,
		changeLevel:    changeLevel,
		unmatchedLevel: unmatchedLevel,
		formatters:     formatters,
		eg:             &eg,
		batches:        make(batchMap),
		formatError:    new(atomic.Bool),
	}, nil
}
