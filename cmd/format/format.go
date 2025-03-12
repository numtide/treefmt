package format

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/numtide/treefmt/v2/config"
	"github.com/numtide/treefmt/v2/format"
	"github.com/numtide/treefmt/v2/stats"
	"github.com/numtide/treefmt/v2/walk"
	"github.com/numtide/treefmt/v2/walk/cache"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	bolt "go.etcd.io/bbolt"
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
		// For humans, the second level precision should not be a problem as they are unlikely to run treefmt in
		// sub-second succession.
		time.Sleep(time.Until(startAfter))
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

	var db *bolt.DB

	// open the db unless --no-cache was specified
	if !cfg.NoCache {
		db, err = cache.Open(cfg.TreeRoot)
		if err != nil {
			return fmt.Errorf("failed to open cache: %w", err)
		}

		// ensure db is closed after we're finished
		defer func() {
			if closeErr := db.Close(); closeErr != nil {
				log.Errorf("failed to close cache: %v", closeErr)
			}
		}()
	}

	if db != nil {
		// clear the cache if desired
		if cfg.ClearCache {
			if err = cache.Clear(db); err != nil {
				return fmt.Errorf("failed to clear cache: %w", err)
			}
		}
	}

	// create an overall app context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// listen for shutdown signal and cancel the context
	go func() {
		exit := make(chan os.Signal, 1)
		signal.Notify(exit, os.Interrupt, syscall.SIGTERM)
		<-exit
		cancel()
	}()

	// parse the walk type
	walkType, err := walk.TypeString(cfg.Walk)
	if err != nil {
		return fmt.Errorf("invalid walk type: %w", err)
	}

	if walkType == walk.Stdin && len(paths) != 1 {
		// check we have only received one path arg which we use for the file extension / matching to formatters
		return errors.New("exactly one path should be specified when using the --stdin flag")
	}

	if err = resolvePaths(paths, walkType, cfg.TreeRoot); err != nil {
		return err
	}

	// create a composite formatter which will handle applying the correct formatters to each file we traverse
	formatter, err := format.NewCompositeFormatter(cfg, statz, BatchSize)
	if err != nil {
		return fmt.Errorf("failed to create composite formatter: %w", err)
	}

	// create a new walker for traversing the paths
	walker, err := walk.NewCompositeReader(walkType, cfg.TreeRoot, paths, db, statz)
	if err != nil {
		return fmt.Errorf("failed to create walker: %w", err)
	}

	// start traversing
	files := make([]*walk.File, BatchSize)

	var (
		n                  int
		readErr, formatErr error
	)

	for {
		// read the next batch
		readCtx, cancelRead := context.WithTimeout(ctx, 1*time.Second)

		n, readErr = walker.Read(readCtx, files)
		log.Debugf("read %d files", n)

		// ensure context is cancelled to release resources
		cancelRead()

		// format any files that were read before processing the read error
		if formatErr = formatter.Apply(ctx, files[:n]); formatErr != nil {
			break
		}

		// stop reading files if there was a read error
		if readErr != nil {
			break
		}
	}

	// finalize formatting (there could be formatting tasks in-flight)
	formatCloseErr := formatter.Close(ctx)

	// close the walker, ensuring any pending file release hooks finish
	walkerCloseErr := walker.Close()

	// print stats to stderr
	if !cfg.Quiet {
		statz.PrintToStderr()
	}

	// process errors

	//nolint:gocritic
	if errors.Is(readErr, io.EOF) {
		// nothing more to read, reset the error and break out of the read loop
		log.Debugf("no more files to read")
	} else if errors.Is(readErr, context.DeadlineExceeded) {
		// the read timed-out
		return errors.New("timeout reading files")
	} else if readErr != nil {
		// something unexpected happened
		return fmt.Errorf("failed to read files: %w", readErr)
	}

	if formatErr != nil {
		return fmt.Errorf("failed to format files: %w", formatErr)
	}

	if formatCloseErr != nil {
		return fmt.Errorf("failed to finalise formatting: %w", formatCloseErr)
	}

	if walkerCloseErr != nil {
		return fmt.Errorf("failed to close walker: %w", walkerCloseErr)
	}

	if cfg.FailOnChange && statz.Value(stats.Changed) != 0 {
		// if fail on change has been enabled, check that no files were actually changed, throwing an error if so
		return ErrFailOnChange
	}

	return nil
}

// resolvePaths checks all paths are contained within the tree root and exist
// also "normalize" paths so they're relative to `cfg.TreeRoot`
// Symlinks are allowed in `paths` and we resolve them here, since
// the readers will ignore symlinks.
func resolvePaths(paths []string, walkType walk.Type, treeRoot string) error {
	for i, path := range paths {
		log.Errorf("Resolving path '%s': %v", path, walkType)

		absolutePath, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("error computing absolute path of %s: %w", path, err)
		}

		if walkType != walk.Stdin {
			realPath, err := filepath.EvalSymlinks(absolutePath)
			if err != nil {
				return fmt.Errorf("path %s not found: %w", absolutePath, err)
			}

			absolutePath = realPath
		}

		relativePath, err := filepath.Rel(treeRoot, absolutePath)
		if err != nil {
			return fmt.Errorf("error computing relative path from %s to %s: %w", treeRoot, absolutePath, err)
		}

		if strings.HasPrefix(relativePath, "..") {
			return fmt.Errorf("path %s not inside the tree root %s", path, treeRoot)
		}

		paths[i] = relativePath
	}

	return nil
}
