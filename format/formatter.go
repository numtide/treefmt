package format

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gobwas/glob"
	"github.com/numtide/treefmt/config"
	"github.com/numtide/treefmt/walk"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
)

var (
	ErrInvalidName = errors.New("formatter name must only contain alphanumeric characters, `_` or `-`")
	// ErrCommandNotFound is returned when the Command for a Formatter is not available.
	ErrCommandNotFound = errors.New("formatter command not found in PATH")

	nameRegex = regexp.MustCompile("^[a-zA-Z0-9_-]+$")
)

// Formatter represents a command which should be applied to a filesystem.
type Formatter struct {
	name   string
	config *config.Formatter

	log        *log.Logger
	executable string // path to the executable described by Command
	workingDir string

	// internal, compiled versions of Includes and Excludes.
	includes []glob.Glob
	excludes []glob.Glob
}

func (f *Formatter) Name() string {
	return f.name
}

func (f *Formatter) Priority() int {
	return f.config.Priority
}

// Executable returns the path to the executable defined by Command.
func (f *Formatter) Executable() string {
	return f.executable
}

func (f *Formatter) Apply(ctx context.Context, files []*walk.File) error {
	start := time.Now()

	// construct args, starting with config
	args := f.config.Options

	// exit early if nothing to process
	if len(files) == 0 {
		return nil
	}

	// append paths to the args
	for _, file := range files {
		args = append(args, file.RelPath)
	}

	// execute the command
	cmd := exec.CommandContext(ctx, f.executable, args...) //nolint:gosec
	// replace the default Cancel handler installed by CommandContext because it sends SIGKILL (-9).
	cmd.Cancel = func() error {
		return cmd.Process.Signal(os.Interrupt)
	}
	cmd.Dir = f.workingDir

	// log out the command being executed
	f.log.Debugf("executing: %s", cmd.String())

	if out, err := cmd.CombinedOutput(); err != nil {
		f.log.Errorf("failed to apply with options '%v': %s", f.config.Options, err)

		if len(out) > 0 {
			_, _ = fmt.Fprintf(os.Stderr, "\n%s\n", out)
		}

		return fmt.Errorf("formatter '%s' with options '%v' failed to apply: %w", f.config.Command, f.config.Options, err)
	}

	f.log.Infof("%v file(s) processed in %v", len(files), time.Since(start))

	return nil
}

// Wants is used to determine if a Formatter wants to process a path based on it's configured Includes and Excludes
// patterns.
// Returns true if the Formatter should be applied to file, false otherwise.
func (f *Formatter) Wants(file *walk.File) bool {
	match := !pathMatches(file.RelPath, f.excludes) && pathMatches(file.RelPath, f.includes)
	if match {
		f.log.Debugf("match: %v", file)
	}

	return match
}

// newFormatter is used to create a new Formatter.
func newFormatter(
	name string,
	treeRoot string,
	env expand.Environ,
	cfg *config.Formatter,
) (*Formatter, error) {
	var err error

	// check the name is valid
	if !nameRegex.MatchString(name) {
		return nil, ErrInvalidName
	}

	f := Formatter{}

	// capture config and the formatter's name
	f.name = name
	f.config = cfg
	f.workingDir = treeRoot

	// test if the formatter is available
	executable, err := interp.LookPathDir(treeRoot, env, cfg.Command)
	if err != nil {
		return nil, ErrCommandNotFound
	}

	f.executable = executable

	// initialise internal state
	if cfg.Priority > 0 {
		f.log = log.WithPrefix(fmt.Sprintf("format | %s[%d]", name, cfg.Priority))
	} else {
		f.log = log.WithPrefix(fmt.Sprintf("format | %s", name))
	}

	f.includes, err = compileGlobs(cfg.Includes)
	if err != nil {
		return nil, fmt.Errorf("failed to compile formatter '%v' includes: %w", f.name, err)
	}

	f.excludes, err = compileGlobs(cfg.Excludes)
	if err != nil {
		return nil, fmt.Errorf("failed to compile formatter '%v' excludes: %w", f.name, err)
	}

	return &f, nil
}
