package format

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gobwas/glob"
)

// ErrFormatterNotFound is returned when the Command for a Formatter is not available.
var ErrFormatterNotFound = errors.New("formatter not found")

// Formatter represents a command which should be applied to a filesystem.
type Formatter struct {
	// Command is the command invoke when applying this Formatter.
	Command string
	// Options are an optional list of args to be passed to Command.
	Options []string
	// Includes is a list of glob patterns used to determine whether this Formatter should be applied against a path.
	Includes []string
	// Excludes is an optional list of glob patterns used to exclude certain files from this Formatter.
	Excludes []string

	name       string
	log        *log.Logger
	executable string // path to the executable described by Command

	// internal compiled versions of Includes and Excludes.
	includes []glob.Glob
	excludes []glob.Glob

	// inbox is used to accept new paths for formatting.
	inbox chan string

	// Entries from inbox are batched according to batchSize and stored in batch for processing when the batchSize has
	// been reached or Close is invoked.
	batch     []string
	batchSize int
}

// Executable returns the path to the executable defined by Command
func (f *Formatter) Executable() string {
	return f.executable
}

// Init is used to initialise internal state before this Formatter is ready to accept paths.
func (f *Formatter) Init(name string) error {
	// capture the name from the config file
	f.name = name

	// test if the formatter is available
	executable, err := exec.LookPath(f.Command)
	if errors.Is(err, exec.ErrNotFound) {
		return ErrFormatterNotFound
	} else if err != nil {
		return err
	}
	f.executable = executable

	// initialise internal state
	f.log = log.WithPrefix("format | " + name)
	f.batchSize = 1024
	f.inbox = make(chan string, f.batchSize)
	f.batch = make([]string, f.batchSize)
	f.batch = f.batch[:0]

	// todo refactor common code below
	if len(f.Includes) > 0 {
		for _, pattern := range f.Includes {
			g, err := glob.Compile("**/" + pattern)
			if err != nil {
				return fmt.Errorf("%w: failed to compile include pattern '%v' for formatter '%v'", err, pattern, f.name)
			}
			f.includes = append(f.includes, g)
		}
	}

	if len(f.Excludes) > 0 {
		for _, pattern := range f.Excludes {
			g, err := glob.Compile("**/" + pattern)
			if err != nil {
				return fmt.Errorf("%w: failed to compile exclude pattern '%v' for formatter '%v'", err, pattern, f.name)
			}
			f.excludes = append(f.excludes, g)
		}
	}

	return nil
}

// Wants is used to test if a Formatter wants path based on it's configured Includes and Excludes patterns.
// Returns true if the Formatter should be applied to path, false otherwise.
func (f *Formatter) Wants(path string) bool {
	match := !PathMatches(path, f.excludes) && PathMatches(path, f.includes)
	if match {
		f.log.Debugf("match: %v", path)
	}
	return match
}

// Put add path into this Formatter's inbox for processing.
func (f *Formatter) Put(path string) {
	f.inbox <- path
}

// Run is the main processing loop for this Formatter.
// It accepts a context which is used to lookup certain dependencies and for cancellation.
func (f *Formatter) Run(ctx context.Context) (err error) {
LOOP:
	// keep processing until ctx has been cancelled or inbox has been closed
	for {
		select {

		case <-ctx.Done():
			// ctx has been cancelled
			err = ctx.Err()
			break LOOP

		case path, ok := <-f.inbox:
			// check if the inbox has been closed
			if !ok {
				break LOOP
			}

			// add path to the current batch
			f.batch = append(f.batch, path)

			if len(f.batch) == f.batchSize {
				// drain immediately
				if err := f.apply(ctx); err != nil {
					break LOOP
				}
			}
		}
	}

	// check if LOOP was exited due to an error
	if err != nil {
		return
	}

	// processing any lingering batch
	return f.apply(ctx)
}

// apply executes Command against the latest batch of paths.
// It accepts a context which is used to lookup certain dependencies and for cancellation.
func (f *Formatter) apply(ctx context.Context) error {
	// empty check
	if len(f.batch) == 0 {
		return nil
	}

	// construct args, starting with config
	args := f.Options

	// append each file path
	for _, path := range f.batch {
		args = append(args, path)
	}

	// execute
	start := time.Now()
	cmd := exec.CommandContext(ctx, f.Command, args...)

	if _, err := cmd.CombinedOutput(); err != nil {
		// todo log output
		return err
	}

	f.log.Infof("%v files processed in %v", len(f.batch), time.Now().Sub(start))

	// mark each path in this batch as completed
	for _, path := range f.batch {
		MarkFormatComplete(ctx, path)
	}

	// reset batch
	f.batch = f.batch[:0]

	return nil
}

// Close is used to indicate that a Formatter should process any remaining paths and then stop it's processing loop.
func (f *Formatter) Close() {
	close(f.inbox)
}
