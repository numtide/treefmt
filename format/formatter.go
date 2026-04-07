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
	"github.com/gobwas/glob"
	"github.com/numtide/treefmt/v2/config"
	"github.com/numtide/treefmt/v2/walk"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
)

const (
	BatchSize = 1024
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

func (f *Formatter) MaxBatchSize() int {
	if f.config.MaxBatchSize == nil {
		return BatchSize
	}

	return *f.config.MaxBatchSize
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
	h.Write(fmt.Appendf(nil, "%d %d", info.Size(), info.ModTime().Unix()))

	return nil
}

func (f *Formatter) Apply(ctx context.Context, files []*walk.File) error {
	if len(files) > f.MaxBatchSize() {
		return fmt.Errorf("formatter cannot format %d files at once (max batch size: %d)", len(files), f.MaxBatchSize())
	}

	start := time.Now()

	// construct args, starting with config
	args := f.config.Options

	// exit early if nothing to process
	if len(files) == 0 {
		return nil
	}

	onlyFile := (*walk.File)(nil)
	if len(files) == 1 {
		onlyFile = files[0]
	}

	// If we have a TmpPath, that means we're formatting something other than the  file's RelPath
	// mode". If the underlying formatter has its own stdin mode support, then
	// we can invoke it in a way where we pass along the files's RelPath as an
	// "advisory path", which may affect the behavior of the formatter.
	useStdinMode := files[0].TmpPath != "" && f.config.StdinOptions != nil

	stdin := (*os.File)(nil)

	if useStdinMode {
		// Feed the file to the formatter via stdin.
		tmpFile, err := os.Open(onlyFile.TmpPath)
		if err != nil {
			panic(err)
		}
		defer tmpFile.Close()

		stdin = tmpFile

		replacer := strings.NewReplacer("$path", onlyFile.RelPath)
		for _, arg := range f.config.StdinOptions {
			args = append(args, replacer.Replace(arg))
		}
	} else {
		// append paths to the args
		for _, file := range files {
			if file.TmpPath != "" {
				args = append(args, file.TmpPath)
			} else {
				args = append(args, file.RelPath)
			}
		}
	}

	// execute the command
	cmd := exec.CommandContext(ctx, f.executable, args...) //nolint:gosec
	// replace the default Cancel handler installed by CommandContext because it sends SIGKILL (-9).
	cmd.Cancel = func() error {
		return cmd.Process.Signal(os.Interrupt)
	}
	cmd.Dir = f.workingDir
	cmd.Stdin = stdin

	// log out the command being executed
	f.log.Debugf("executing: %s", cmd.String())

	out, err := cmd.CombinedOutput()
	if err != nil {
		f.log.Errorf("failed to apply with options '%v': %s", f.config.Options, err)

		if len(out) > 0 {
			_, _ = fmt.Fprintf(os.Stderr, "\n%s\n", out)
		}

		return fmt.Errorf("formatter '%s' with options '%v' failed to apply: %w", f.config.Command, f.config.Options, err)
	}

	// In stdin mode, the formatter won't write to the filesystem, it instead
	// prints the formatted file to stdout. Take that output and write it back
	// to the input file (`TmpPath`).
	if useStdinMode {
		path := onlyFile.TmpPath
		if err = os.WriteFile(path, out, 0o600); err != nil {
			return fmt.Errorf("couldn't write to '%s'", path)
		}
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
		return nil, fmt.Errorf("%w: error looking up '%s'", ErrCommandNotFound, cfg.Command)
	}

	f.executable = executable

	// initialise internal state
	if cfg.Priority > 0 {
		f.log = log.WithPrefix(fmt.Sprintf("formatter | %s[%d]", name, cfg.Priority))
	} else {
		f.log = log.WithPrefix("formatter | " + name)
	}

	// check there is at least one include
	if len(cfg.Includes) == 0 {
		return nil, fmt.Errorf("formatter '%v' has no includes", f.name)
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
