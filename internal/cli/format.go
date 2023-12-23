package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/numtide/treefmt/internal/cache"
	"github.com/numtide/treefmt/internal/format"

	"github.com/charmbracelet/log"
	"github.com/juju/errors"
	"github.com/ztrue/shutdown"
	"golang.org/x/sync/errgroup"
)

type Format struct{}

func (f *Format) Run() error {
	start := time.Now()

	Cli.Log.ConfigureLogger()

	l := log.WithPrefix("format")

	defer func() {
		if err := cache.Close(); err != nil {
			l.Errorf("failed to close cache: %v", err)
		}
	}()

	// create an overall context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// register shutdown hook
	shutdown.Add(cancel)

	// read config
	cfg, err := format.ReadConfigFile(Cli.ConfigFile)
	if err != nil {
		return errors.Annotate(err, "failed to read config file")
	}

	// init formatters
	for name, formatter := range cfg.Formatters {
		if err = formatter.Init(name); err != nil {
			return errors.Annotatef(err, "failed to initialise formatter: %v", name)
		}
	}

	ctx = format.RegisterFormatters(ctx, cfg.Formatters)

	if err = cache.Open(Cli.TreeRoot, Cli.ClearCache); err != nil {
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

		var pending, completed int

	LOOP:
		for {
			select {
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
					if err := cache.WriteModTime(batch); err != nil {
						return err
					}
					batch = batch[:0]
				}

				completed += 1

				if completed == pending {
					close(completedCh)
				}
			}
		}

		// final flush
		if err := cache.WriteModTime(batch); err != nil {
			return err
		}

		println(fmt.Sprintf("%v files changed in %v", completed, time.Now().Sub(start)))
		return nil
	})

	eg.Go(func() error {
		count := 0

		for path := range pathsCh {
			// todo cycle detection in Befores
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

	return eg.Wait()
}
