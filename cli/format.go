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
	// create a prefixed logger
	l := log.WithPrefix("format")

	// ensure cache is closed on return
	defer func() {
		if err := cache.Close(); err != nil {
			l.Errorf("failed to close cache: %v", err)
		}
	}()

	// read config
	cfg, err := config.ReadFile(Cli.ConfigFile, Cli.Formatters)
	if err != nil {
		return fmt.Errorf("%w: failed to read config file", err)
	}

	// compile global exclude globs
	if globalExcludes, err = format.CompileGlobs(cfg.Global.Excludes); err != nil {
		return fmt.Errorf("%w: failed to compile global globs", err)
	}

	// initialise pipelines
	pipelines = make(map[string]*format.Pipeline)
	formatters = make(map[string]*format.Formatter)

	for _, name := range cfg.Names {
		formatterCfg := cfg.Formatters[name]
		formatter, err := format.NewFormatter(name, Cli.TreeRoot, formatterCfg, globalExcludes)
		if errors.Is(err, format.ErrCommandNotFound) && Cli.AllowMissingFormatter {
			l.Debugf("formatter not found: %v", name)
			continue
		} else if err != nil {
			return fmt.Errorf("%w: failed to initialise formatter: %v", err, name)
		}

		formatters[name] = formatter

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

	// create some groups for concurrent processing and control flow
	eg, ctx := errgroup.WithContext(ctx)

	// create a channel for paths to be processed
	// we use a multiple of batch size here to allow for greater concurrency
	filesCh = make(chan *walk.File, BatchSize*runtime.NumCPU())

	// create a channel for tracking paths that have been processed
	processedCh = make(chan *walk.File, cap(filesCh))

	// start concurrent processing tasks
	eg.Go(updateCache(ctx))
	eg.Go(applyFormatters(ctx))
	eg.Go(walkFilesystem(ctx))

	// wait for everything to complete
	return eg.Wait()
}

func walkFilesystem(ctx context.Context) func() error {
	return func() error {
		paths := Cli.Paths

		if len(paths) == 0 && Cli.Stdin {

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("%w: failed to determine current working directory", err)
			}

			// read in all the paths
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				path := scanner.Text()
				if !strings.HasPrefix(path, "/") {
					// append the cwd
					path = filepath.Join(cwd, path)
				}

				paths = append(paths, path)
			}
		}

		walker, err := walk.New(Cli.Walk, Cli.TreeRoot, paths)
		if err != nil {
			return fmt.Errorf("failed to create walker: %w", err)
		}

		defer close(filesCh)

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

		if err = cache.ChangeSet(ctx, walker, filesCh); err != nil {
			return fmt.Errorf("failed to generate change set: %w", err)
		}
		return nil
	}
}

func updateCache(ctx context.Context) func() error {
	return func() error {
		batch := make([]*walk.File, 0, BatchSize)

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
			case <-ctx.Done():
				return ctx.Err()
			case path, ok := <-processedCh:
				if !ok {
					break LOOP
				}
				batch = append(batch, path)
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

		if Cli.FailOnChange && stats.Value(stats.Formatted) != 0 {
			return ErrFailOnChange
		}

		stats.Print()
		return nil
	}
}

func applyFormatters(ctx context.Context) func() error {
	fg, ctx := errgroup.WithContext(ctx)
	batches := make(map[string][]*walk.File)

	tryApply := func(key string, file *walk.File) {
		batch, ok := batches[key]
		if !ok {
			batch = make([]*walk.File, 0, BatchSize)
		}
		batch = append(batch, file)
		batches[key] = batch

		if len(batch) == BatchSize {
			pipeline := pipelines[key]

			// copy the batch
			files := make([]*walk.File, len(batch))
			copy(files, batch)

			fg.Go(func() error {
				if err := pipeline.Apply(ctx, files); err != nil {
					return err
				}
				for _, path := range files {
					processedCh <- path
				}
				return nil
			})

			batches[key] = batch[:0]
		}
	}

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

		for file := range filesCh {
			var matched bool
			for key, pipeline := range pipelines {
				if !pipeline.Wants(file) {
					continue
				}
				matched = true
				tryApply(key, file)
			}
			if matched {
				stats.Add(stats.Matched, 1)
			} else {
				processedCh <- file
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
