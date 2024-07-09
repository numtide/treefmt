package format

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"git.numtide.com/numtide/treefmt/walker"

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

	//

	f.log.Infof("%v file(s) processed in %v", len(tasks), time.Since(start))

	return nil
}

// Wants is used to test if a Formatter wants a path based on it's configured Includes and Excludes patterns.
// Returns true if the Formatter should be applied to path, false otherwise.
func (f *Formatter) Wants(file *walker.File) bool {
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
