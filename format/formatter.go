package format

import (
	"context"
	"errors"
	"fmt"
	"hash"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/numtide/treefmt/v2/config"
	"github.com/numtide/treefmt/v2/matcher"
	"github.com/numtide/treefmt/v2/walk"
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

	// internal, compiled versions of inclusion and exclusion matchers.
	matchFn matcher.MatchFn
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

// Hash adds this formatter's config and executable info to the config hash being created.
func (f *Formatter) Hash(h hash.Hash) error {
	// including the name helps us to easily detect when formatters have been added/removed
	h.Write([]byte(f.name))
	// if options change, the outcome of applying the formatter might be different
	h.Write([]byte(strings.Join(f.config.Options, " ")))
	// if priority changes, the outcome of applying a sequence of formatters might be different
	h.Write([]byte(strconv.Itoa(f.config.Priority)))

	// stat the formatter's executable
	info, err := os.Lstat(f.executable)
	if err != nil {
		return fmt.Errorf("failed to stat formatter executable: %w", err)
	}

	// include the executable's size and mod time
	// if the formatter executable changes (e.g. new version) the outcome of applying the formatter might differ
	h.Write([]byte(fmt.Sprintf("%d %d", info.Size(), info.ModTime().Unix())))

	return nil
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
		if file.TmpPath != "" {
			args = append(args, file.TmpPath)
		} else {
			args = append(args, file.RelPath)
		}
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

// Wants is used to determine if a Formatter wants to process a path based on
// its configured Includes and Excludes patterns and Select and Reject
// templates.
func (f *Formatter) Wants(file *walk.File) (matcher.Result, error) {
	result, err := f.matchFn(file)
	if err != nil {
		return result, fmt.Errorf("error applying matcher to %s: %w", file.RelPath, err)
	}

	if result == matcher.Wanted {
		f.log.Debugf("match: %v", file)
	}

	return result, nil
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
		return nil, fmt.Errorf("%w: error looking up '%s'", ErrCommandNotFound, cfg.Command)
	}

	f.executable = executable

	// initialise internal state
	if cfg.Priority > 0 {
		f.log = log.WithPrefix(fmt.Sprintf("formatter | %s[%d]", name, cfg.Priority))
	} else {
		f.log = log.WithPrefix("formatter | " + name)
	}

	// check there is at least one include or select filter
	if len(cfg.Includes) == 0 && len(cfg.Select) == 0 {
		return nil, fmt.Errorf("formatter '%v' has no includes or select filters", f.name)
	}

	includes := make([]matcher.MatchFn, 2)
	excludes := make([]matcher.MatchFn, 2)

	includes[0], err = matcher.IncludeGlobs(cfg.Includes)
	if err != nil {
		return nil, fmt.Errorf("failed to compile formatter '%v' includes: %w", f.name, err)
	}

	includes[1], err = matcher.IncludeTemplates(cfg.Select)
	if err != nil {
		return nil, fmt.Errorf("failed to compile formatter '%v' select: %w", f.name, err)
	}

	excludes[0], err = matcher.ExcludeGlobs(cfg.Excludes)
	if err != nil {
		return nil, fmt.Errorf("failed to compile formatter '%v' excludes: %w", f.name, err)
	}

	excludes[1], err = matcher.ExcludeTemplates(cfg.Reject)
	if err != nil {
		return nil, fmt.Errorf("failed to compile formatter '%v' reject: %w", f.name, err)
	}

	f.matchFn = matcher.Combine(includes, excludes)

	return &f, nil
}
