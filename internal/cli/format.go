package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"git.numtide.com/numtide/treefmt/internal/cache"
	"git.numtide.com/numtide/treefmt/internal/format"

	"github.com/charmbracelet/log"
	"golang.org/x/sync/errgroup"
)

type Format struct{}

func (f *Format) Run() error {
	start := time.Now()

	Cli.Configure()

	l := log.WithPrefix("format")

	defer func() {
		if err := cache.Close(); err != nil {
			l.Errorf("failed to close cache: %v", err)
		}
	}()

	// create an overall context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// read config
	cfg, err := format.ReadConfigFile(Cli.ConfigFile)
	if err != nil {
		return fmt.Errorf("%w: failed to read config file", err)
	}

	globalExcludes, err := format.CompileGlobs(cfg.Global.Excludes)

	// create optional formatter filter set
	formatterSet := make(map[string]bool)
	for _, name := range Cli.Formatters {
		_, ok := cfg.Formatters[name]
		if !ok {
			return fmt.Errorf("%w: formatter not found in config: %v", err, name)
		}
		formatterSet[name] = true
	}

	includeFormatter := func(name string) bool {
		if len(formatterSet) == 0 {
			return true
		} else {
			_, include := formatterSet[name]
			return include
		}
	}

	// init formatters
	for name, formatter := range cfg.Formatters {
		if !includeFormatter(name) {
			// remove this formatter
			delete(cfg.Formatters, name)
			l.Debugf("formatter %v is not in formatter list %v, skipping", name, Cli.Formatters)
			continue
		}

		err = formatter.Init(name, globalExcludes)
		if errors.Is(err, format.ErrFormatterNotFound) && Cli.AllowMissingFormatter {
			l.Debugf("formatter not found: %v", name)
			// remove this formatter
			delete(cfg.Formatters, name)
		} else if err != nil {
			return fmt.Errorf("%w: failed to initialise formatter: %v", err, name)
		}
	}

	ctx = format.RegisterFormatters(ctx, cfg.Formatters)

	if err = cache.Open(Cli.TreeRoot, Cli.ClearCache, cfg.Formatters); err != nil {
		return err
	}

	//
	pendingCh := make(chan string, 1024)
	completedCh := make(chan string, 1024)

	ctx = format.SetCompletedChannel(ctx, completedCh)

	//
	eg, ctx := errgroup.WithContext(ctx)

	// start the formatters
	for name := range cfg.Formatters {
		formatter := cfg.Formatters[name]
		eg.Go(func() error {
			return formatter.Run(ctx)
		})
	}

	// determine paths to be formatted
	pathsCh := make(chan string, 1024)

	// update cache as paths are completed
	eg.Go(func() error {
		batchSize := 1024
		batch := make([]string, batchSize)
		batch = batch[:0]

		var pending, completed, changes int

	LOOP:
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case _, ok := <-pendingCh:
				if ok {
					pending += 1
				} else if pending == completed {
					break LOOP
				}

			case path, ok := <-completedCh:
				if !ok {
					break LOOP
				}
				batch = append(batch, path)
				if len(batch) == batchSize {
					count, err := cache.Update(batch)
					if err != nil {
						return err
					}
					changes += count
					batch = batch[:0]
				}

				completed += 1

				if completed == pending {
					close(completedCh)
				}
			}
		}

		// final flush
		count, err := cache.Update(batch)
		if err != nil {
			return err
		}
		changes += count

		fmt.Printf("%v files changed in %v", changes, time.Now().Sub(start))
		return nil
	})

	eg.Go(func() error {
		count := 0

		for path := range pathsCh {
			for _, formatter := range cfg.Formatters {
				if formatter.Wants(path) {
					pendingCh <- path
					count += 1
					formatter.Put(path)
				}
			}
		}

		for _, formatter := range cfg.Formatters {
			formatter.Close()
		}

		if count == 0 {
			close(completedCh)
		}

		return nil
	})

	eg.Go(func() error {
		defer close(pathsCh)
		return cache.ChangeSet(ctx, Cli.TreeRoot, pathsCh)
	})

	// listen for shutdown and call cancel if required
	go func() {
		exit := make(chan os.Signal, 1)
		signal.Notify(exit, os.Interrupt, syscall.SIGTERM)
		<-exit
		cancel()
	}()

	return eg.Wait()
}
