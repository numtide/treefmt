package format

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/charmbracelet/log"
	"github.com/gobwas/glob"
	"github.com/numtide/treefmt/config"
	"github.com/numtide/treefmt/stats"
	"github.com/numtide/treefmt/walk"
	"mvdan.cc/sh/v3/expand"
)

const (
	batchKeySeparator = ":"
)

var ErrFormattingFailures = errors.New("formatting failures detected")

// CompositeFormatter handles the application of multiple Formatter instances based on global excludes and individual
// formatter configuration.
type CompositeFormatter struct {
	cfg            *config.Config
	stats          *stats.Stats
	globalExcludes []glob.Glob

	unmatchedLevel log.Level

	scheduler  *scheduler
	formatters map[string]*Formatter
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

	// iterate the formatters, recording which are interested in this file
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

		if accepted, err := c.scheduler.submit(ctx, file, matches); err != nil {
			return fmt.Errorf("failed to schedule file: %w", err)
		} else if !accepted {
			// if a file wasn't accepted, it means there was no formatting to perform
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

// signature generates a formatting signature, which is a combination of the signatures for each of the formatters
// we delegate to.
func (c *CompositeFormatter) signature() (signature, error) {
	h := sha256.New()

	// sort formatters deterministically
	formatters := make([]*Formatter, 0, len(c.formatters))
	for _, f := range c.formatters {
		formatters = append(formatters, f)
	}

	slices.SortFunc(formatters, formatterSortFunc)

	// apply them to the hash
	for _, f := range formatters {
		if err := f.Hash(h); err != nil {
			return nil, fmt.Errorf("failed to hash formatter: %w", err)
		}
	}

	// finalize
	return h.Sum(nil), nil
}

// Close finalizes the processing of the CompositeFormatter, ensuring that any remaining batches are applied and
// all formatters have completed their tasks. It returns an error if any formatting failures were detected.
func (c *CompositeFormatter) Close(ctx context.Context) error {
	return c.scheduler.close(ctx)
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

	// create a scheduler for carrying out the actual formatting
	scheduler := newScheduler(statz, batchSize, changeLevel, formatters)

	return &CompositeFormatter{
		cfg:            cfg,
		stats:          statz,
		globalExcludes: globalExcludes,
		unmatchedLevel: unmatchedLevel,

		scheduler:  scheduler,
		formatters: formatters,
	}, nil
}
