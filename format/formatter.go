package format

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"golang.org/x/sync/errgroup"

	"git.numtide.com/numtide/treefmt/walk"

	"git.numtide.com/numtide/treefmt/config"

	"github.com/charmbracelet/log"
	"github.com/gobwas/glob"
)

// ErrCommandNotFound is returned when the Command for a Formatter is not available.
var ErrCommandNotFound = errors.New("formatter command not found in PATH")

// Formatter represents a command which should be applied to a filesystem.
type Formatter struct {
	name   string
	config *config.Formatter

	log        *log.Logger
	executable string // path to the executable described by Command
	workingDir string

	// internal compiled versions of Includes and Excludes.
	includes []glob.Glob
	excludes []glob.Glob
}

// Executable returns the path to the executable defined by Command
func (f *Formatter) Executable() string {
	return f.executable
}

func (f *Formatter) Name() string {
	return f.name
}

func (f *Formatter) Priority() int {
	return f.config.Priority
}

func (f *Formatter) Apply(ctx context.Context, tasks []*Task) error {
	start := time.Now()
	defer func() {
		f.log.Infof("%v files processed in %v", len(tasks), time.Since(start))
	}()

	if f.config.BatchSize == 0 {
		// apply as one single batch
		return f.applyBatch(ctx, tasks)
	}

	// otherwise we create smaller batches and apply them concurrently
	// todo how to constrain the overall number of 'threads' as we want a separate group here for the eg.Wait()
	eg := errgroup.Group{}

	var batch []*Task
	for idx := range tasks {
		batch = append(batch, tasks[idx])
		if len(batch) == f.config.BatchSize {
			// copy the batch as we re-use it for the next batch
			next := make([]*Task, len(batch))
			copy(next, batch)

			// fire off a routine to process the next batch
			eg.Go(func() error {
				return f.applyBatch(ctx, next)
			})
			// reset batch for next iteration
			batch = batch[:0]
		}
	}

	// flush final partial batch
	if len(batch) > 0 {
		eg.Go(func() error {
			return f.applyBatch(ctx, batch)
		})
	}

	return eg.Wait()
}

func (f *Formatter) applyBatch(ctx context.Context, tasks []*Task) error {
	f.log.Debugf("applying batch, size = %d", len(tasks))

	// construct args, starting with config
	args := f.config.Options

	// exit early if nothing to process
	if len(tasks) == 0 {
		return nil
	}

	// append paths to the args
	for _, task := range tasks {
		args = append(args, task.File.RelPath)
	}

	// execute the command
	cmd := exec.CommandContext(ctx, f.executable, args...)
	// replace the default Cancel handler installed by CommandContext because it sends SIGKILL (-9).
	cmd.Cancel = func() error {
		return cmd.Process.Signal(os.Interrupt)
	}
	cmd.Dir = f.workingDir

	// log out the command being executed
	f.log.Debugf("executing: %s", cmd.String())

	if out, err := cmd.CombinedOutput(); err != nil {
		if len(out) > 0 {
			_, _ = fmt.Fprintf(os.Stderr, "%s error:\n%s\n", f.name, out)
		}
		return fmt.Errorf("formatter '%s' with options '%v' failed to apply: %w", f.config.Command, f.config.Options, err)
	}

	return nil
}

// Wants is used to test if a Formatter wants a path based on it's configured Includes and Excludes patterns.
// Returns true if the Formatter should be applied to path, false otherwise.
func (f *Formatter) Wants(file *walk.File) bool {
	match := !PathMatches(file.RelPath, f.excludes) && PathMatches(file.RelPath, f.includes)
	if match {
		f.log.Debugf("match: %v", file)
	}
	return match
}

// NewFormatter is used to create a new Formatter.
func NewFormatter(
	name string,
	treeRoot string,
	cfg *config.Formatter,
) (*Formatter, error) {
	var err error

	f := Formatter{}

	// capture config and the formatter's name
	f.name = name
	f.config = cfg
	f.workingDir = treeRoot

	// test if the formatter is available
	executable, err := exec.LookPath(cfg.Command)
	if errors.Is(err, exec.ErrNotFound) {
		return nil, ErrCommandNotFound
	} else if err != nil {
		return nil, err
	}
	f.executable = executable

	// initialise internal state
	if cfg.Priority > 0 {
		f.log = log.WithPrefix(fmt.Sprintf("format | %s[%d]", name, cfg.Priority))
	} else {
		f.log = log.WithPrefix(fmt.Sprintf("format | %s", name))
	}

	f.includes, err = CompileGlobs(cfg.Includes)
	if err != nil {
		return nil, fmt.Errorf("failed to compile formatter '%v' includes: %w", f.name, err)
	}

	f.excludes, err = CompileGlobs(cfg.Excludes)
	if err != nil {
		return nil, fmt.Errorf("failed to compile formatter '%v' excludes: %w", f.name, err)
	}

	return &f, nil
}
