package format

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"syscall"
	"time"

	"github.com/numtide/treefmt/walk/cache"
	bolt "go.etcd.io/bbolt"

	"github.com/charmbracelet/log"
	"github.com/gobwas/glob"
	"github.com/numtide/treefmt/config"
	"github.com/numtide/treefmt/format"
	"github.com/numtide/treefmt/stats"
	"github.com/numtide/treefmt/walk"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"

	"mvdan.cc/sh/v3/expand"
)

const (
	BatchSize = 1024
)

var ErrFailOnChange = errors.New("unexpected changes detected, --fail-on-change is enabled")

func Run(v *viper.Viper, statz *stats.Stats, cmd *cobra.Command, paths []string) error {
	cmd.SilenceUsage = true

	cfg, err := config.FromViper(v)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.CI {
		log.Info("ci mode enabled")

		startAfter := time.Now().
			// truncate to second precision
			Truncate(time.Second).
			// add one second
			Add(1 * time.Second).
			// a little extra to ensure we don't start until the next second
			Add(10 * time.Millisecond)

		log.Debugf("waiting until %v before continuing", startAfter)

		// Wait until we tick over into the next second before processing to ensure our EPOCH level modtime comparisons
		// for change detection are accurate.
		// This can fail in CI between checkout and running treefmt if everything happens too quickly.
		// For humans, the second level precision should not be a problem as they are unlikely to run treefmt in sub-second succession.
		<-time.After(time.Until(startAfter))
	}

	// cpu profiling
	if cfg.CpuProfile != "" {
		cpuProfile, err := os.Create(cfg.CpuProfile)
		if err != nil {
			return fmt.Errorf("failed to open file for writing cpu profile: %w", err)
		} else if err = pprof.StartCPUProfile(cpuProfile); err != nil {
			return fmt.Errorf("failed to start cpu profile: %w", err)
		}
		defer func() {
			pprof.StopCPUProfile()
			if err := cpuProfile.Close(); err != nil {
				log.Errorf("failed to close cpu profile: %v", err)
			}
		}()
	}

	// create a prefixed logger
	log.SetPrefix("format")

	// compile global exclude globs
	globalExcludes, err := format.CompileGlobs(cfg.Excludes)
	if err != nil {
		return fmt.Errorf("failed to compile global excludes: %w", err)
	}

	// initialise formatters
	formatters := make(map[string]*format.Formatter)

	env := expand.ListEnviron(os.Environ()...)

	for name, formatterCfg := range cfg.FormatterConfigs {
		formatter, err := format.NewFormatter(name, cfg.TreeRoot, env, formatterCfg)

		if errors.Is(err, format.ErrCommandNotFound) && cfg.AllowMissingFormatter {
			log.Debugf("formatter command not found: %v", name)
			continue
		} else if err != nil {
			return fmt.Errorf("%w: failed to initialise formatter: %v", err, name)
		}

		// store formatter by name
		formatters[name] = formatter
	}

	var db *bolt.DB

	if !cfg.NoCache {
		// open the db
		db, err = cache.Open(cfg.TreeRoot)
		if err != nil {
			return fmt.Errorf("failed to open cache: %w", err)
		}

		// ensure db is closed after we're finished
		defer func() {
			if err := db.Close(); err != nil {
				log.Errorf("failed to close cache: %v", err)
			}
		}()

		// clear the cache if desired
		if cfg.ClearCache {
			if err = cache.Clear(db); err != nil {
				return fmt.Errorf("failed to clear cache: %w", err)
			}
		}

		// Compare formatters, clearing paths if they have changed, and recording their latest info in the db
		if err = format.CompareFormatters(db, formatters); err != nil {
			return fmt.Errorf("failed to compare formatters: %w", err)
		}
	}

	// create an app context and listen for shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		exit := make(chan os.Signal, 1)
		signal.Notify(exit, os.Interrupt, syscall.SIGTERM)
		<-exit
		cancel()
	}()

	// create an overall error group for executing high level tasks concurrently
	eg, ctx := errgroup.WithContext(ctx)

	// create a channel for files needing to be processed
	// we use a multiple of batch size here as a rudimentary concurrency optimization based on the host machine
	filesCh := make(chan *walk.File, BatchSize*runtime.NumCPU())

	// create a channel for files that have been formatted
	formattedCh := make(chan *format.Task, cap(filesCh))

	// start concurrent processing tasks in reverse order
	eg.Go(postProcessing(ctx, cfg, statz, formattedCh))
	eg.Go(applyFormatters(ctx, cfg, statz, globalExcludes, formatters, filesCh, formattedCh))

	// parse the walk type
	walkType, err := walk.TypeString(cfg.Walk)
	if err != nil {
		return fmt.Errorf("invalid walk type: %w", err)
	}

	if walkType == walk.Stdin {
		// check we have only received one path arg which we use for the file extension / matching to formatters
		if len(paths) != 1 {
			return fmt.Errorf("exactly one path should be specified when using the --stdin flag")
		}
	}

	// checks all paths are contained within the tree root
	for _, path := range paths {
		rootPath := filepath.Join(cfg.TreeRoot, path)
		if _, err = os.Stat(rootPath); err != nil {
			return fmt.Errorf("path %s not found within the tree root %s", path, cfg.TreeRoot)
		}
	}

	// create a new reader for traversing the paths
	reader, err := walk.NewCompositeReader(walkType, cfg.TreeRoot, paths, db, statz)
	if err != nil {
		return fmt.Errorf("failed to create walker: %w", err)
	}

	//

	files := make([]*walk.File, BatchSize)
	for {
		ctx, cancel := context.WithTimeout(ctx, 1*time.Second)

		n, err := reader.Read(ctx, files)

		for idx := 0; idx < n; idx++ {
			file := files[idx]

			// check if this file is new or has changed when compared to the cache entry
			if file.Cache == nil || file.Cache.HasChanged(file.Info) {
				filesCh <- file
				statz.Add(stats.Emitted, 1)
			}
		}

		cancel()

		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			log.Errorf("failed to read files: %v", err)
			cancel()
			break
		}
	}

	close(filesCh)

	// wait for everything to complete
	if err = eg.Wait(); err != nil {
		return err
	}

	return reader.Close()
}

// applyFormatters
func applyFormatters(
	ctx context.Context,
	cfg *config.Config,
	statz *stats.Stats,
	globalExcludes []glob.Glob,
	formatters map[string]*format.Formatter,
	filesCh chan *walk.File,
	formattedCh chan *format.Task,
) func() error {
	// create our own errgroup for concurrent formatting tasks.
	// we don't want a cancel clause, in order to let formatters run up to the end.
	fg := errgroup.Group{}
	// simple optimization to avoid too many concurrent formatting tasks
	// we can queue them up faster than the formatters can process them, this paces things a bit
	fg.SetLimit(runtime.NumCPU())

	// track batches of formatting task based on their batch keys, which are determined by the unique sequence of
	// formatters which should be applied to their respective files
	batches := make(map[string][]*format.Task)

	apply := func(key string, flush bool) {
		// lookup the batch and exit early if it's empty
		batch := batches[key]
		if len(batch) == 0 {
			return
		}

		// process the batch if it's full, or we've been asked to flush partial batches
		if flush || len(batch) == BatchSize {

			// copy the batch as we re-use it for the next batch
			tasks := make([]*format.Task, len(batch))
			copy(tasks, batch)

			// asynchronously apply the sequence formatters to the batch
			fg.Go(func() error {
				// Iterate the formatters, applying them in sequence to the batch of tasks.
				// We get the formatter list from the first task since they have all the same formatters list.
				formatters := tasks[0].Formatters

				var formatErrors []error
				for idx := range formatters {
					if err := formatters[idx].Apply(ctx, tasks); err != nil {
						formatErrors = append(formatErrors, err)
					}
				}

				// pass each file to the formatted channel
				for _, task := range tasks {
					task.Errors = formatErrors
					formattedCh <- task
				}

				return nil
			})

			// reset the batch
			batches[key] = batch[:0]
		}
	}

	tryApply := func(task *format.Task) {
		// append to batch
		key := task.BatchKey
		batches[key] = append(batches[key], task)
		// try to apply
		apply(key, false)
	}

	return func() error {
		defer func() {
			// close processed channel
			close(formattedCh)
		}()

		unmatchedLevel, err := log.ParseLevel(cfg.OnUnmatched)
		if err != nil {
			return fmt.Errorf("invalid on-unmatched value: %w", err)
		}

		// iterate the files channel
		for file := range filesCh {

			// first check if this file has been globally excluded
			if format.PathMatches(file.RelPath, globalExcludes) {
				log.Debugf("path matched global excludes: %s", file.RelPath)
				// mark it as processed and continue to the next
				formattedCh <- &format.Task{
					File: file,
				}
				continue
			}

			// check if any formatters are interested in this file
			var matches []*format.Formatter
			for _, formatter := range formatters {
				if formatter.Wants(file) {
					matches = append(matches, formatter)
				}
			}

			// see if any formatters matched
			if len(matches) == 0 {

				if unmatchedLevel == log.FatalLevel {
					return fmt.Errorf("no formatter for path: %s", file.RelPath)
				}
				log.Logf(unmatchedLevel, "no formatter for path: %s", file.RelPath)
				// mark it as processed and continue to the next
				formattedCh <- &format.Task{
					File: file,
				}
			} else {
				// record the match
				statz.Add(stats.Matched, 1)
				// create a new format task, add it to a batch based on its batch key and try to apply if the batch is full
				task := format.NewTask(file, matches)
				tryApply(&task)
			}
		}

		// flush any partial batches which remain
		for key := range batches {
			apply(key, true)
		}

		// wait for all outstanding formatting tasks to complete
		if err := fg.Wait(); err != nil {
			return fmt.Errorf("formatting failure: %w", err)
		}
		return nil
	}
}

func postProcessing(
	ctx context.Context,
	cfg *config.Config,
	statz *stats.Stats,
	formattedCh chan *format.Task,
) func() error {
	return func() error {
	LOOP:
		for {
			select {

			// detect ctx cancellation
			case <-ctx.Done():
				return ctx.Err()

			// take the next task that has been processed
			case task, ok := <-formattedCh:
				if !ok {
					break LOOP
				}

				// check if the file has changed
				file := task.File
				changed, newInfo, err := file.Stat()
				if err != nil {
					return err
				}

				if changed {
					// record the change
					statz.Add(stats.Formatted, 1)

					logMethod := log.Debug
					if cfg.FailOnChange {
						// surface the changed file more obviously
						logMethod = log.Error
					}

					// log the change
					logMethod(
						"file has changed",
						"path", file.RelPath,
						"prev_size", file.Info.Size(),
						"prev_mod_time", file.Info.ModTime().Truncate(time.Second),
						"current_size", newInfo.Size(),
						"current_mod_time", newInfo.ModTime().Truncate(time.Second),
					)
					// update the file info
					file.Info = newInfo
				}

				if err := file.Release(); err != nil {
					return fmt.Errorf("failed to release file: %w", err)
				}
			}
		}

		// if fail on change has been enabled, check that no files were actually formatted, throwing an error if so
		if cfg.FailOnChange && statz.Value(stats.Formatted) != 0 {
			return ErrFailOnChange
		}

		// print stats to stdout unless we are processing stdin and printing the results to stdout
		if !cfg.Stdin {
			statz.Print()
		}

		return nil
	}
}
