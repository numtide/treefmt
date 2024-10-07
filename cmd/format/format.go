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

	"github.com/charmbracelet/log"
	"github.com/gobwas/glob"
	"github.com/numtide/treefmt/cache"
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

func Run(v *viper.Viper, cmd *cobra.Command, paths []string) error {
	cmd.SilenceUsage = true

	cfg, err := config.FromViper(v)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// initialise stats collection
	stats.Init()

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
		// For humans, the second level precision should not be a problem as they are unlikely to run treefmt in
		// sub-second succession.
		<-time.After(time.Until(startAfter))
	}

	// for stdin, check we have only received one path arg which we use for the file extension / matching to formatters
	if cfg.Stdin && len(paths) != 1 {
		return fmt.Errorf("exactly one path should be specified when using the --stdin flag")
	}

	if cfg.Stdin {
		// read stdin into a temporary file with the same file extension
		pattern := fmt.Sprintf("*%s", filepath.Ext(paths[0]))

		file, err := os.CreateTemp("", pattern)
		if err != nil {
			return fmt.Errorf("failed to create a temporary file for processing stdin: %w", err)
		}

		if _, err = io.Copy(file, os.Stdin); err != nil {
			return fmt.Errorf("failed to copy stdin into a temporary file")
		}

		// set the tree root to match the temp directory
		cfg.TreeRoot, err = filepath.Abs(filepath.Dir(file.Name()))
		if err != nil {
			return fmt.Errorf("failed to get absolute path for tree root: %w", err)
		}

		// configure filesystem walker to traverse the temporary tree root
		cfg.Walk = "filesystem"

		// update paths with temp file
		paths[0] = file.Name()
	} else {
		// checks all paths are contained within the tree root
		for idx, path := range paths {
			rootPath := filepath.Join(cfg.TreeRoot, path)
			if _, err = os.Stat(rootPath); err != nil {
				return fmt.Errorf("path %s not found within the tree root %s", path, cfg.TreeRoot)
			}
			// update the path entry with an absolute path
			paths[idx] = filepath.Clean(rootPath)
		}
	}

	// cpu profiling
	if cfg.CPUProfile != "" {
		cpuProfile, err := os.Create(cfg.CPUProfile)
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

	// ensure cache is closed on return
	defer func() {
		if err := cache.Close(); err != nil {
			log.Errorf("failed to close cache: %v", err)
		}
	}()

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

	// open the cache if configured
	if !cfg.NoCache {
		if err = cache.Open(cfg.TreeRoot, cfg.ClearCache, formatters); err != nil {
			// if we can't open the cache, we log a warning and fallback to no cache
			log.Warnf("failed to open cache: %v", err)

			cfg.NoCache = true
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

	// create a channel for files that have been processed
	processedCh := make(chan *format.Task, cap(filesCh))

	// start concurrent processing tasks in reverse order
	eg.Go(updateCache(ctx, cfg, processedCh))
	eg.Go(detectFormatted(ctx, cfg, formattedCh, processedCh))
	eg.Go(applyFormatters(ctx, cfg, globalExcludes, formatters, filesCh, formattedCh))
	eg.Go(walkFilesystem(ctx, cfg, paths, filesCh))

	// wait for everything to complete
	return eg.Wait()
}

func walkFilesystem(
	ctx context.Context,
	cfg *config.Config,
	paths []string,
	filesCh chan *walk.File,
) func() error {
	return func() error {
		// close the files channel when we're done walking the file system
		defer close(filesCh)

		eg, ctx := errgroup.WithContext(ctx)
		pathsCh := make(chan string, BatchSize)

		// By default, we use the cli arg, but if the stdin flag has been set we force a filesystem walk
		// since we will only be processing one file from a temp directory
		walkerType, err := walk.TypeString(cfg.Walk)
		if err != nil {
			return fmt.Errorf("invalid walk type: %w", err)
		}

		walkPaths := func() error {
			defer close(pathsCh)

			var idx int
			for idx < len(paths) {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
					pathsCh <- paths[idx]

					idx++
				}
			}

			return nil
		}

		if len(paths) > 0 {
			eg.Go(walkPaths)
		} else {
			// no explicit paths to process, so we only need to process root
			pathsCh <- cfg.TreeRoot
			close(pathsCh)
		}

		// create a filesystem walker
		walker, err := walk.New(walkerType, cfg.TreeRoot, pathsCh)
		if err != nil {
			return fmt.Errorf("failed to create walker: %w", err)
		}

		// if no cache has been configured, or we are processing from stdin, we invoke the walker directly
		if cfg.NoCache || cfg.Stdin {
			return walker.Walk(ctx, func(file *walk.File, _ error) error {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
					stats.Add(stats.Traversed, 1)
					stats.Add(stats.Emitted, 1)
					filesCh <- file

					return nil
				}
			})
		}

		// otherwise we pass the walker to the cache and have it generate files for processing based on whether or not
		// they have been added/changed since the last invocation
		if err = cache.ChangeSet(ctx, walker, filesCh); err != nil {
			return fmt.Errorf("failed to generate change set: %w", err)
		}

		return nil
	}
}

// applyFormatters.
func applyFormatters(
	ctx context.Context,
	cfg *config.Config,
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
				stats.Add(stats.Matched, 1)
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

func detectFormatted(
	ctx context.Context,
	cfg *config.Config,
	formattedCh chan *format.Task,
	processedCh chan *format.Task,
) func() error {
	return func() error {
		defer func() {
			// close formatted channel
			close(processedCh)
		}()

		for {
			select {
			// detect ctx cancellation
			case <-ctx.Done():
				return ctx.Err()
			// take the next task that has been processed
			case task, ok := <-formattedCh:
				if !ok {
					// channel has been closed, no further files to process
					return nil
				}

				// check if the file has changed
				file := task.File

				changed, newInfo, err := file.HasChanged()
				if err != nil {
					return err
				}

				if changed {
					// record the change
					stats.Add(stats.Formatted, 1)

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

				// mark as processed
				processedCh <- task
			}
		}
	}
}

func updateCache(ctx context.Context, cfg *config.Config, processedCh chan *format.Task) func() error {
	return func() error {
		// used to batch updates for more efficient txs
		batch := make([]*format.Task, 0, BatchSize)

		// apply a batch
		processBatch := func() error {
			// pass the batch to the cache for updating
			files := make([]*walk.File, len(batch))
			for idx := range batch {
				files[idx] = batch[idx].File
			}

			if err := cache.Update(files); err != nil {
				return err
			}

			batch = batch[:0]

			return nil
		}

		// if we are processing from stdin that means we are outputting to stdout, no caching involved
		// if f.NoCache is set that means either the user explicitly disabled the cache or we failed to open on
		if cfg.Stdin || cfg.NoCache {
			// do nothing
			processBatch = func() error { return nil }
		}

	LOOP:
		for {
			select {
			// detect ctx cancellation
			case <-ctx.Done():
				return ctx.Err()
			// respond to formatted files
			case task, ok := <-processedCh:
				if !ok {
					// channel has been closed, no further files to process
					break LOOP
				}

				file := task.File

				if cfg.Stdin {
					// dump file into stdout
					f, err := os.Open(file.Path)
					if err != nil {
						return fmt.Errorf("failed to open %s: %w", file.Path, err)
					}
					if _, err = io.Copy(os.Stdout, f); err != nil {
						return fmt.Errorf("failed to copy %s to stdout: %w", file.Path, err)
					}
					if err = os.Remove(f.Name()); err != nil {
						return fmt.Errorf("failed to remove temp file %s: %w", file.Path, err)
					}

					continue
				}

				// Append to batch and process if we have enough.
				// We do not cache any files that were part of a pipeline in which one or more formatters failed.
				// This is to ensure those files are re-processed in later invocations after the user has potentially
				// resolved the issue, e.g. fixed a config problem.
				if len(task.Errors) == 0 {
					batch = append(batch, task)
					if len(batch) == BatchSize {
						if err := processBatch(); err != nil {
							return err
						}
					}
				}
			}
		}

		// final flush
		if err := processBatch(); err != nil {
			return err
		}

		// if fail on change has been enabled, check that no files were actually formatted, throwing an error if so
		if cfg.FailOnChange && stats.Value(stats.Formatted) != 0 {
			return ErrFailOnChange
		}

		// print stats to stdout unless we are processing stdin and printing the results to stdout
		if !cfg.Stdin {
			stats.Print()
		}

		return nil
	}
}
