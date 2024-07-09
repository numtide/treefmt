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
	"time"

	"git.numtide.com/numtide/treefmt/format"
	"git.numtide.com/numtide/treefmt/stats"

	"git.numtide.com/numtide/treefmt/cache"
	"git.numtide.com/numtide/treefmt/config"
	"git.numtide.com/numtide/treefmt/walker"

	"github.com/charmbracelet/log"
	"golang.org/x/sync/errgroup"
)

const (
	BatchSize = 1024
)

var ErrFailOnChange = errors.New("unexpected changes detected, --fail-on-change is enabled")

func (f *Format) Run() (err error) {
	// set log level and other options
	f.configureLogging()

	// cpu profiling
	if f.CpuProfile != "" {
		cpuProfile, err := os.Create(f.CpuProfile)
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
	if f.ConfigFile == "" {
		pwd, err := os.Getwd()
		if err != nil {
			return err
		}
		f.ConfigFile, _, err = findUp(pwd, "treefmt.toml")
		if err != nil {
			return err
		}
	}

	// default the tree root to the directory containing the config file
	if f.TreeRoot == "" {
		f.TreeRoot = filepath.Dir(f.ConfigFile)
	}

	// search the tree root using the --tree-root-file if specified
	if f.TreeRootFile != "" {
		pwd, err := os.Getwd()
		if err != nil {
			return err
		}
		_, f.TreeRoot, err = findUp(pwd, f.TreeRootFile)
		if err != nil {
			return err
		}
	}

	log.Debugf("config-file=%s tree-root=%s", f.ConfigFile, f.TreeRoot)

	// read config
	cfg, err := config.ReadFile(f.ConfigFile, f.Formatters)
	if err != nil {
		return fmt.Errorf("failed to read config file %v: %w", f.ConfigFile, err)
	}

	// compile global exclude globs
	if f.globalExcludes, err = format.CompileGlobs(cfg.Global.Excludes); err != nil {
		return fmt.Errorf("failed to compile global excludes: %w", err)
	}

	// initialise formatters
	f.formatters = make(map[string]*format.Formatter)

	for name, formatterCfg := range cfg.Formatters {
		formatter, err := format.NewFormatter(name, f.TreeRoot, formatterCfg)

		if errors.Is(err, format.ErrCommandNotFound) && f.AllowMissingFormatter {
			log.Debugf("formatter command not found: %v", name)
			continue
		} else if err != nil {
			return fmt.Errorf("%w: failed to initialise formatter: %v", err, name)
		}

		// store formatter by name
		f.formatters[name] = formatter
	}

	// open the cache if configured
	if !f.NoCache {
		if err = cache.Open(f.TreeRoot, f.ClearCache, f.formatters); err != nil {
			// if we can't open the cache, we log a warning and fallback to no cache
			log.Warnf("failed to open cache: %v", err)
			f.NoCache = true
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

	// initialise stats collection
	stats.Init()

	// create an overall error group for executing high level tasks concurrently
	eg, ctx := errgroup.WithContext(ctx)

	// create a channel for files needing to be processed
	// we use a multiple of batch size here as a rudimentary concurrency optimization based on the host machine
	f.fileCh = make(chan *walker.File, BatchSize*runtime.NumCPU())

	// create a channel for files that have been formatted
	f.formattedCh = make(chan *walker.File, cap(f.fileCh))

	// create a channel for files that have been processed
	f.processedCh = make(chan *walker.File, cap(f.fileCh))

	// start concurrent processing tasks in reverse order
	eg.Go(f.updateCache(ctx))
	eg.Go(f.detectFormatted(ctx))
	eg.Go(f.applyFormatters(ctx))
	eg.Go(f.walkFilesystem(ctx))

	// wait for everything to complete
	return eg.Wait()
}

func (f *Format) walkFilesystem(ctx context.Context) func() error {
	return func() error {
		eg, ctx := errgroup.WithContext(ctx)
		pathCh := make(chan string, BatchSize)

		// By default, we use the cli arg, but if the stdin flag has been set we force a filesystem walk
		// since we will only be processing one file from a temp directory
		walkerType := f.Walk

		if f.Stdin {
			walkerType = walker.Filesystem

			// check we have only received one path arg which we use for the file extension / matching to formatters
			if len(f.Paths) != 1 {
				return fmt.Errorf("only one path should be specified when using the --stdin flag")
			}

			// read stdin into a temporary file with the same file extension
			pattern := fmt.Sprintf("*%s", filepath.Ext(f.Paths[0]))
			file, err := os.CreateTemp("", pattern)
			if err != nil {
				return fmt.Errorf("failed to create a temporary file for processing stdin: %w", err)
			}

			if _, err = io.Copy(file, os.Stdin); err != nil {
				return fmt.Errorf("failed to copy stdin into a temporary file")
			}

			f.Paths[0] = file.Name()
		}

		walkPaths := func() error {
			defer close(pathCh)

			var idx int
			for idx < len(f.Paths) {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
					pathCh <- f.Paths[idx]
					idx += 1
				}
			}

			return nil
		}

		if len(f.Paths) > 0 {
			eg.Go(walkPaths)
		} else {
			// no explicit paths to process, so we only need to process root
			pathCh <- f.TreeRoot
			close(pathCh)
		}

		// create a filesystem walker
		wk, err := walker.New(walkerType, f.TreeRoot, f.NoCache, pathCh)
		if err != nil {
			return fmt.Errorf("failed to create walker: %w", err)
		}

		// close the file channel when we're done walking the file system
		defer close(f.fileCh)

		// if no cache has been configured, or we are processing from stdin, we invoke the walker directly
		if f.NoCache || f.Stdin {
			return wk.Walk(ctx, func(file *walker.File, err error) error {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
					stats.Add(stats.Traversed, 1)
					stats.Add(stats.Emitted, 1)
					f.fileCh <- file
					return nil
				}
			})
		}

		// otherwise we pass the walker to the cache and have it generate files for processing based on whether or not
		// they have been added/changed since the last invocation
		if err = cache.ChangeSet(ctx, wk, f.fileCh); err != nil {
			return fmt.Errorf("failed to generate change set: %w", err)
		}
		return nil
	}
}

// applyFormatters
func (f *Format) applyFormatters(ctx context.Context) func() error {
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
				// iterate the formatters, applying them in sequence to the batch of tasks
				// we get the formatters list from the first task since they have all the same formatters list
				for _, f := range tasks[0].Formatters {
					if err := f.Apply(ctx, tasks); err != nil {
						return err
					}
				}

				// pass each file to the formatted channel
				for _, task := range tasks {
					f.formattedCh <- task.File
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
			close(f.formattedCh)
		}()

		// iterate the files channel
		for file := range f.fileCh {

			// first check if this file has been globally excluded
			if format.PathMatches(file.RelPath, f.globalExcludes) {
				log.Debugf("path matched global excludes: %s", file.RelPath)
				// mark it as processed and continue to the next
				f.formattedCh <- file
				continue
			}

			// check if any formatters are interested in this file
			var matches []*format.Formatter
			for _, formatter := range f.formatters {
				if formatter.Wants(file) {
					matches = append(matches, formatter)
				}
			}

			// see if any formatters matched
			if len(matches) == 0 {
				if f.OnUnmatched == log.FatalLevel {
					return fmt.Errorf("no formatter for path: %s", file.RelPath)
				}
				log.Logf(f.OnUnmatched, "no formatter for path: %s", file.RelPath)
				// mark it as processed and continue to the next
				f.formattedCh <- file
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

func (f *Format) detectFormatted(ctx context.Context) func() error {
	return func() error {
		defer func() {
			// close formatted channel
			close(f.processedCh)
		}()

		for {
			select {

			// detect ctx cancellation
			case <-ctx.Done():
				return ctx.Err()
			// take the next file that has been processed
			case file, ok := <-f.formattedCh:
				if !ok {
					// channel has been closed, no further files to process
					return nil
				}

				// check if the file has changed
				changed, newInfo, err := file.HasChanged()
				if err != nil {
					return err
				}

				if changed {
					// record the change
					stats.Add(stats.Formatted, 1)
					// log the change for diagnostics
					log.Debug(
						"file has changed",
						"path", file.Path,
						"prev_size", file.Info.Size(),
						"current_size", newInfo.Size(),
						"prev_mod_time", file.Info.ModTime().Truncate(time.Second),
						"current_mod_time", newInfo.ModTime().Truncate(time.Second),
					)
					// update the file info
					file.Info = newInfo
				}

				// mark as processed
				f.processedCh <- file
			}
		}
	}
}

func (f *Format) updateCache(ctx context.Context) func() error {
	return func() error {
		// used to batch updates for more efficient txs
		batch := make([]*walker.File, 0, BatchSize)

		// apply a batch
		processBatch := func() error {
			// pass the batch to the cache for updating
			if err := cache.Update(batch); err != nil {
				return err
			}
			batch = batch[:0]
			return nil
		}

		// if we are processing from stdin that means we are outputting to stdout, no caching involved
		// if f.NoCache is set that means either the user explicitly disabled the cache or we failed to open on
		if f.Stdin || f.NoCache {
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
			case file, ok := <-f.processedCh:
				if !ok {
					// channel has been closed, no further files to process
					break LOOP
				}

				if f.Stdin {
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
		if f.FailOnChange && stats.Value(stats.Formatted) != 0 {
			return ErrFailOnChange
		}

		// print stats to stdout unless we are processing stdin and printing the results to stdout
		if !f.Stdin {
			stats.Print()
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
