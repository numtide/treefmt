package format

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

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

	batch []string
}

// Executable returns the path to the executable defined by Command
func (f *Formatter) Executable() string {
	return f.executable
}

func (f *Formatter) Apply(ctx context.Context, files []*walk.File, filter bool) error {
	start := time.Now()

	// construct args, starting with config
	args := f.config.Options

	// If filter is true it indicates we are executing as part of a pipeline.
	// In such a scenario each formatter must sub filter the paths provided as different formatters might want different
	// files in a pipeline.
	if filter {
		// reset the batch
		f.batch = f.batch[:0]

		// filter paths
		for _, file := range files {
			if f.Wants(file) {
				f.batch = append(f.batch, file.RelPath)
			}
		}

		// exit early if nothing to process
		if len(f.batch) == 0 {
			return nil
		}

		// append paths to the args
		args = append(args, f.batch...)
	} else {
		// exit early if nothing to process
		if len(files) == 0 {
			return nil
		}

		// append paths to the args
		for _, file := range files {
			args = append(args, file.RelPath)
		}
	}

	// execute the command
	cmd := exec.CommandContext(ctx, f.config.Command, args...)
	cmd.Dir = f.workingDir

	if out, err := cmd.CombinedOutput(); err != nil {
		if len(out) > 0 {
			_, _ = fmt.Fprintf(os.Stderr, "%s error:\n%s\n", f.name, out)
		}
		return fmt.Errorf("formatter '%s' with options '%v' failed to apply: %w", f.config.Command, f.config.Options, err)
	}

	//

	f.log.Infof("%v files processed in %v", len(files), time.Now().Sub(start))

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
	globalExcludes []glob.Glob,
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
	if cfg.Pipeline == "" {
		f.log = log.WithPrefix(fmt.Sprintf("format | %s", name))
	} else {
		f.log = log.WithPrefix(fmt.Sprintf("format | %s[%s]", cfg.Pipeline, name))
	}

	f.includes, err = CompileGlobs(cfg.Includes)
	if err != nil {
		return nil, fmt.Errorf("failed to compile formatter '%v' includes: %w", f.name, err)
	}

	f.excludes, err = CompileGlobs(cfg.Excludes)
	if err != nil {
		return nil, fmt.Errorf("failed to compile formatter '%v' excludes: %w", f.name, err)
	}
	f.excludes = append(f.excludes, globalExcludes...)

	return &f, nil
}
