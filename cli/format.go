package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"time"

	"git.numtide.com/numtide/treefmt/format"
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
	start          time.Time
	globalExcludes []glob.Glob
	formatters     map[string]*format.Formatter
	pipelines      map[string]*format.Pipeline
	pathsCh        chan string
	processedCh    chan string

	ErrFailOnChange = errors.New("unexpected changes detected, --fail-on-change is enabled")
)

func (f *Format) Run() (err error) {
	start = time.Now()

	Cli.Configure()

	l := log.WithPrefix("format")

	defer func() {
		if err := cache.Close(); err != nil {
			l.Errorf("failed to close cache: %v", err)
		}
	}()

	// read config
	cfg, err := config.ReadFile(Cli.ConfigFile)
	if err != nil {
		return fmt.Errorf("%w: failed to read config file", err)
	}

	if globalExcludes, err = format.CompileGlobs(cfg.Global.Excludes); err != nil {
		return fmt.Errorf("%w: failed to compile global globs", err)
	}

	pipelines = make(map[string]*format.Pipeline)
	formatters = make(map[string]*format.Formatter)

	// filter formatters
	if len(Cli.Formatters) > 0 {
		// first check the cli formatter list is valid
		for _, name := range Cli.Formatters {
			_, ok := cfg.Formatters[name]
			if !ok {
				return fmt.Errorf("formatter not found in config: %v", name)
			}
		}
		// next we remove any formatter configs that were not specified
		for name := range cfg.Formatters {
			if !slices.Contains(Cli.Formatters, name) {
				delete(cfg.Formatters, name)
			}
		}
	}

	// init formatters
	for name, formatterCfg := range cfg.Formatters {
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

	// create some groups for concurrent processing and control flow
	eg, ctx := errgroup.WithContext(ctx)

	// create a channel for paths to be processed
	// we use a multiple of batch size here to allow for greater concurrency
	pathsCh = make(chan string, BatchSize*runtime.NumCPU())

	// create a channel for tracking paths that have been processed
	processedCh = make(chan string, cap(pathsCh))

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

		defer close(pathsCh)

		if Cli.NoCache {
			return walker.Walk(ctx, func(path string, info fs.FileInfo, err error) error {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
					// ignore symlinks and directories
					if !(info.IsDir() || info.Mode()&os.ModeSymlink == os.ModeSymlink) {
						pathsCh <- path
					}
					return nil
				}
			})
		}

		if err = cache.ChangeSet(ctx, walker, pathsCh); err != nil {
			return fmt.Errorf("failed to generate change set: %w", err)
		}
		return nil
	}
}

func updateCache(ctx context.Context) func() error {
	return func() error {
		batch := make([]string, 0, BatchSize)

		var changes int

		processBatch := func() error {
			if Cli.NoCache {
				changes += len(batch)
			} else {
				count, err := cache.Update(Cli.TreeRoot, batch)
				if err != nil {
					return err
				}
				changes += count
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

		if Cli.FailOnChange && changes != 0 {
			return ErrFailOnChange
		}

		fmt.Printf("%v files changed in %v\n", changes, time.Now().Sub(start))
		return nil
	}
}

func applyFormatters(ctx context.Context) func() error {
	fg, ctx := errgroup.WithContext(ctx)
	batches := make(map[string][]string)

	tryApply := func(key string, path string) {
		batch, ok := batches[key]
		if !ok {
			batch = make([]string, 0, BatchSize)
		}
		batch = append(batch, path)
		batches[key] = batch

		if len(batch) == BatchSize {
			pipeline := pipelines[key]

			// copy the batch
			paths := make([]string, len(batch))
			copy(paths, batch)

			fg.Go(func() error {
				if err := pipeline.Apply(ctx, paths); err != nil {
					return err
				}
				for _, path := range paths {
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

		for path := range pathsCh {
			for key, pipeline := range pipelines {
				if !pipeline.Wants(path) {
					continue
				}
				tryApply(key, path)
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
