package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"syscall"

	"git.numtide.com/numtide/treefmt/format"
	"git.numtide.com/numtide/treefmt/stats"
	"github.com/gobwas/glob"

	"git.numtide.com/numtide/treefmt/cache"
	"git.numtide.com/numtide/treefmt/config"
	"git.numtide.com/numtide/treefmt/walk"

	"github.com/charmbracelet/log"
	"golang.org/x/sync/errgroup"
)

const (
	BatchSize = 1024
)

var (
	globalExcludes []glob.Glob
	formatters     map[string]*format.Formatter
	pipelines      map[string]*format.Pipeline
	filesCh        chan *walk.File
	processedCh    chan *walk.File

	ErrFailOnChange = errors.New("unexpected changes detected, --fail-on-change is enabled")
)

func (f *Format) Run() (err error) {
	// cpu profiling
	if Cli.CpuProfile != "" {
		cpuProfile, err := os.Create(Cli.CpuProfile)
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

	// read config
	cfg, err := config.ReadFile(Cli.ConfigFile, Cli.Formatters)
	if err != nil {
		return fmt.Errorf("failed to read config file %v: %w", Cli.ConfigFile, err)
	}

	// compile global exclude globs
	if globalExcludes, err = format.CompileGlobs(cfg.Global.Excludes); err != nil {
		return fmt.Errorf("failed to compile global excludes: %w", err)
	}

	// initialise pipelines
	pipelines = make(map[string]*format.Pipeline)
	formatters = make(map[string]*format.Formatter)

	// iterate the formatters in lexicographical order
	for _, name := range cfg.Names {
		// init formatter
		formatterCfg := cfg.Formatters[name]
		formatter, err := format.NewFormatter(name, Cli.TreeRoot, formatterCfg, globalExcludes)
		if errors.Is(err, format.ErrCommandNotFound) && Cli.AllowMissingFormatter {
			log.Debugf("formatter not found: %v", name)
			continue
		} else if err != nil {
			return fmt.Errorf("%w: failed to initialise formatter: %v", err, name)
		}

		// store formatter by name
		formatters[name] = formatter

		// If no pipeline is configured, we add the formatter to a nominal pipeline of size 1 with the key being the
		// formatter's name. If a pipeline is configured, we add the formatter to a pipeline keyed by
		// 'p:<pipeline_name>' in which it is sorted by priority.
		if formatterCfg.Pipeline == "" {
			pipeline := format.Pipeline{}
			pipeline.Add(formatter)
			pipelines[name] = &pipeline
		} else {
			key := fmt.Sprintf("p:%s", formatterCfg.Pipeline)
			pipeline, ok := pipelines[key]
			if !ok {
				pipeline = &format.Pipeline{}
				pipelines[key] = pipeline
			}
			pipeline.Add(formatter)
		}
	}

	// open the cache
	if err = cache.Open(Cli.TreeRoot, Cli.ClearCache, formatters); err != nil {
		return err
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

	// initialise stats collection
	stats.Init()

	// create an overall error group for executing high level tasks concurrently
	eg, ctx := errgroup.WithContext(ctx)

	// create a channel for files needing to be processed
	// we use a multiple of batch size here as a rudimentary concurrency optimization based on the host machine
	filesCh = make(chan *walk.File, BatchSize*runtime.NumCPU())

	// create a channel for files that have been processed
	processedCh = make(chan *walk.File, cap(filesCh))

	// start concurrent processing tasks in reverse order
	eg.Go(updateCache(ctx))
	eg.Go(applyFormatters(ctx))
	eg.Go(walkFilesystem(ctx))

	// wait for everything to complete
	return eg.Wait()
}

func updateCache(ctx context.Context) func() error {
	return func() error {
		// used to batch updates for more efficient txs
		batch := make([]*walk.File, 0, BatchSize)

		// apply a batch
		processBatch := func() error {
			if err := cache.Update(batch); err != nil {
				return err
			}
			batch = batch[:0]
			return nil
		}

	LOOP:
		for {
			select {
			// detect ctx cancellation
			case <-ctx.Done():
				return ctx.Err()
			// respond to processed files
			case file, ok := <-processedCh:
				if !ok {
					// channel has been closed, no further files to process
					break LOOP
				}
				// append to batch and process if we have enough
				batch = append(batch, file)
				if len(batch) == BatchSize {
					if err := processBatch(); err != nil {
						return err
					}
				}
			}
		}

		// final flush
		if err := processBatch(); err != nil {
			return err
		}

		// if fail on change has been enabled, check that no files were actually formatted, throwing an error if so
		if Cli.FailOnChange && stats.Value(stats.Formatted) != 0 {
			return ErrFailOnChange
		}

		// print stats to stdout
		stats.Print()

		return nil
	}
}

func walkFilesystem(ctx context.Context) func() error {
	return func() error {
		eg, ctx := errgroup.WithContext(ctx)
		pathsCh := make(chan string, BatchSize)

		walkPaths := func() error {
			defer close(pathsCh)

			var idx int
			for idx < len(Cli.Paths) {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
					pathsCh <- Cli.Paths[idx]
					idx += 1
				}
			}

			return nil
		}

		walkStdin := func() error {
			defer close(pathsCh)

			// determine the current working directory
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to determine current working directory: %w", err)
			}

			// read in all the paths
			scanner := bufio.NewScanner(os.Stdin)

			for scanner.Scan() {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
					path := scanner.Text()
					if !strings.HasPrefix(path, "/") {
						// append the cwd
						path = filepath.Join(cwd, path)
					}
					pathsCh <- path
				}
			}
			return nil
		}

		if len(Cli.Paths) > 0 {
			eg.Go(walkPaths)
		} else if Cli.Stdin {
			eg.Go(walkStdin)
		} else {
			// no explicit paths to process, so we only need to process root
			pathsCh <- Cli.TreeRoot
			close(pathsCh)
		}

		// create a filesystem walker
		walker, err := walk.New(Cli.Walk, Cli.TreeRoot, pathsCh)
		if err != nil {
			return fmt.Errorf("failed to create walker: %w", err)
		}

		// close the files channel when we're done walking the file system
		defer close(filesCh)

		// if no cache has been configured, we invoke the walker directly
		if Cli.NoCache {
			return walker.Walk(ctx, func(file *walk.File, err error) error {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
					// ignore symlinks and directories
					if !(file.Info.IsDir() || file.Info.Mode()&os.ModeSymlink == os.ModeSymlink) {
						stats.Add(stats.Traversed, 1)
						stats.Add(stats.Emitted, 1)
						filesCh <- file
					}
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

func applyFormatters(ctx context.Context) func() error {
	// create our own errgroup for concurrent formatting tasks
	fg, ctx := errgroup.WithContext(ctx)

	// pre-initialise batches keyed by pipeline
	batches := make(map[string][]*walk.File)
	for key := range pipelines {
		batches[key] = make([]*walk.File, 0, BatchSize)
	}

	// for a given pipeline key, add the provided file to the current batch and trigger a format if the batch size has
	// been reached
	tryApply := func(key string, file *walk.File) {
		// append to batch
		batches[key] = append(batches[key], file)

		// check if the batch is full
		batch := batches[key]
		if len(batch) == BatchSize {
			// get the pipeline
			pipeline := pipelines[key]

			// copy the batch
			files := make([]*walk.File, len(batch))
			copy(files, batch)

			// apply to the pipeline
			fg.Go(func() error {
				if err := pipeline.Apply(ctx, files); err != nil {
					return err
				}
				for _, path := range files {
					processedCh <- path
				}
				return nil
			})

			// reset the batch
			batches[key] = batch[:0]
		}
	}

	// format any partial batches
	flushBatches := func() {
		for key, pipeline := range pipelines {

			batch := batches[key]
			pipeline := pipeline // capture for closure

			if len(batch) > 0 {
				fg.Go(func() error {
					if err := pipeline.Apply(ctx, batch); err != nil {
						return fmt.Errorf("%s failure: %w", key, err)
					}
					for _, path := range batch {
						processedCh <- path
					}
					return nil
				})
			}
		}
	}

	return func() error {
		defer func() {
			// close processed channel
			close(processedCh)
		}()

		// iterate the files channel, checking if any pipeline wants it, and attempting to apply if so.
		for file := range filesCh {
			var matches []string

			for key, pipeline := range pipelines {
				if !pipeline.Wants(file) {
					continue
				}
				matches = append(matches, key)
				tryApply(key, file)
			}
			switch len(matches) {
			case 0:
				log.Debugf("no match found: %s", file.Path)
				// no match, so we send it direct to the processed channel
				processedCh <- file
			case 1:
				stats.Add(stats.Matched, 1)
			default:
				return fmt.Errorf("path '%s' matched multiple formatters/pipelines %v", file.Path, matches)
			}
		}

		// flush any partial batches which remain
		flushBatches()

		// wait for all outstanding formatting tasks to complete
		if err := fg.Wait(); err != nil {
			return fmt.Errorf("pipeline processing failure: %w", err)
		}
		return nil
	}
}
