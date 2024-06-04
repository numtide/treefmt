package cli

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
	excludes   []glob.Glob
	formatters map[string]*format.Formatter

	filesCh     chan *walk.File
	processedCh chan *walk.File

	ErrFailOnChange = errors.New("unexpected changes detected, --fail-on-change is enabled")
)

func (f *Format) Run() (err error) {
	// set log level and other options
	configureLogging()

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

	// find the config file unless specified
	if Cli.ConfigFile == "" {
		pwd, err := os.Getwd()
		if err != nil {
			return err
		}
		Cli.ConfigFile, _, err = findUp(pwd, "treefmt.toml")
		if err != nil {
			return err
		}
	}

	// default the tree root to the directory containing the config file
	if Cli.TreeRoot == "" {
		Cli.TreeRoot = filepath.Dir(Cli.ConfigFile)
	}

	// search the tree root using the --tree-root-file if specified
	if Cli.TreeRootFile != "" {
		pwd, err := os.Getwd()
		if err != nil {
			return err
		}
		_, Cli.TreeRoot, err = findUp(pwd, Cli.TreeRootFile)
		if err != nil {
			return err
		}
	}

	log.Debugf("config-file=%s tree-root=%s", Cli.ConfigFile, Cli.TreeRoot)

	// read config
	cfg, err := config.ReadFile(Cli.ConfigFile, Cli.Formatters)
	if err != nil {
		return fmt.Errorf("failed to read config file %v: %w", Cli.ConfigFile, err)
	}

	// compile global exclude globs
	if excludes, err = format.CompileGlobs(cfg.Global.Excludes); err != nil {
		return fmt.Errorf("failed to compile global excludes: %w", err)
	}

	// initialise formatters
	formatters = make(map[string]*format.Formatter)

	for name, formatterCfg := range cfg.Formatters {
		formatter, err := format.NewFormatter(name, Cli.TreeRoot, formatterCfg, excludes)

		if errors.Is(err, format.ErrCommandNotFound) && Cli.AllowMissingFormatter {
			log.Debugf("formatter command not found: %v", name)
			continue
		} else if err != nil {
			return fmt.Errorf("%w: failed to initialise formatter: %v", err, name)
		}

		// store formatter by name
		formatters[name] = formatter
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
			if Cli.Stdin {
				// do nothing
				return nil
			}
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

				if Cli.Stdin {
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

					stats.Add(stats.Formatted, 1)
					continue
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

		// print stats to stdout unless we are processing stdin and printing the results to stdout
		if !Cli.Stdin {
			stats.Print()
		}

		return nil
	}
}

func walkFilesystem(ctx context.Context) func() error {
	return func() error {
		eg, ctx := errgroup.WithContext(ctx)
		pathsCh := make(chan string, BatchSize)

		// By default, we use the cli arg, but if the stdin flag has been set we force a filesystem walk
		// since we will only be processing one file from a temp directory
		walkerType := Cli.Walk

		if Cli.Stdin {
			walkerType = walk.Filesystem

			// check we have only received one path arg which we use for the file extension / matching to formatters
			if len(Cli.Paths) != 1 {
				return fmt.Errorf("only one path should be specified when using the --stdin flag")
			}

			// read stdin into a temporary file with the same file extension
			pattern := fmt.Sprintf("*%s", filepath.Ext(Cli.Paths[0]))
			file, err := os.CreateTemp("", pattern)
			if err != nil {
				return fmt.Errorf("failed to create a temporary file for processing stdin: %w", err)
			}

			if _, err = io.Copy(file, os.Stdin); err != nil {
				return fmt.Errorf("failed to copy stdin into a temporary file")
			}

			Cli.Paths[0] = file.Name()
		}

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

		if len(Cli.Paths) > 0 {
			eg.Go(walkPaths)
		} else {
			// no explicit paths to process, so we only need to process root
			pathsCh <- Cli.TreeRoot
			close(pathsCh)
		}

		// create a filesystem walker
		walker, err := walk.New(walkerType, Cli.TreeRoot, pathsCh)
		if err != nil {
			return fmt.Errorf("failed to create walker: %w", err)
		}

		// close the files channel when we're done walking the file system
		defer close(filesCh)

		// if no cache has been configured, or we are processing from stdin, we invoke the walker directly
		if Cli.NoCache || Cli.Stdin {
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
				// iterate the formatters, applying them in sequence to the batch of tasks
				// we get the formatters list from the first task since they have all the same formatters list
				for _, f := range tasks[0].Formatters {
					if err := f.Apply(ctx, tasks); err != nil {
						return err
					}
				}

				// pass each file to the processed channel
				for _, task := range tasks {
					processedCh <- task.File
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
			close(processedCh)
		}()

		// iterate the files channel
		for file := range filesCh {

			// determine a list of formatters that are interested in file
			var matches []*format.Formatter
			for _, formatter := range formatters {
				if formatter.Wants(file) {
					matches = append(matches, formatter)
				}
			}

			if len(matches) == 0 {
				if Cli.OnUnmatched == log.FatalLevel {
					return fmt.Errorf("no formatter for path: %s", file.Path)
				}
				log.Logf(Cli.OnUnmatched, "no formatter for path: %s", file.Path)
				processedCh <- file
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

func findUp(searchDir string, fileName string) (path string, dir string, err error) {
	for _, dir := range eachDir(searchDir) {
		path := filepath.Join(dir, fileName)
		if fileExists(path) {
			return path, dir, nil
		}
	}
	return "", "", fmt.Errorf("could not find %s in %s", fileName, searchDir)
}

func eachDir(path string) (paths []string) {
	path, err := filepath.Abs(path)
	if err != nil {
		return
	}

	paths = []string{path}

	if path == "/" {
		return
	}

	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == os.PathSeparator {
			path = path[:i]
			if path == "" {
				path = "/"
			}
			paths = append(paths, path)
		}
	}

	return
}

func fileExists(path string) bool {
	// Some broken filesystems like SSHFS return file information on stat() but
	// then cannot open the file. So we use os.Open.
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	// Next, check that the file is a regular file.
	fi, err := f.Stat()
	if err != nil {
		return false
	}

	return fi.Mode().IsRegular()
}
