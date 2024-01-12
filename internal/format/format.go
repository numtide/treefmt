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

type FormatterConfig struct {
	// Command is the command invoke when applying this Formatter.
	Command string
	// Options are an optional list of args to be passed to Command.
	Options []string
	// Includes is a list of glob patterns used to determine whether this Formatter should be applied against a path.
	Includes []string
	// Excludes is an optional list of glob patterns used to exclude certain files from this Formatter.
	Excludes []string
	// Before is the name of another formatter which must process a path after this one
	Before string
}

// Formatter represents a command which should be applied to a filesystem.
type Formatter struct {
	name   string
	config *FormatterConfig

	log        *log.Logger
	executable string // path to the executable described by Command

	before string
	parent *Formatter
	child  *Formatter

	// internal compiled versions of Includes and Excludes.
	includes []glob.Glob
	excludes []glob.Glob

	// inboxCh is used to accept new paths for formatting.
	inboxCh chan string
	// completedCh is used to wait for this formatter to finish all processing.
	completedCh chan interface{}

	// Entries from inboxCh are batched according to batchSize and stored in batch for processing when the batchSize has
	// been reached or Close is invoked.
	batch     []string
	batchSize int
}

func (f *Formatter) Before() string {
	return f.before
}

func (f *Formatter) ResetBefore() {
	f.before = ""
}

// Executable returns the path to the executable defined by Command
func (f *Formatter) Executable() string {
	return f.executable
}

// NewFormatter is used to create a new Formatter.
func NewFormatter(name string, config *FormatterConfig, globalExcludes []glob.Glob) (*Formatter, error) {
	var err error

	f := Formatter{}
	// capture the name from the config file
	f.name = name
	f.config = config
	f.before = config.Before

	// test if the formatter is available
	executable, err := exec.LookPath(config.Command)
	if errors.Is(err, exec.ErrNotFound) {
		return nil, ErrFormatterNotFound
	} else if err != nil {
		return nil, err
	}
	f.executable = executable

	// initialise internal state
	f.log = log.WithPrefix("format | " + name)
	f.batchSize = 1024
	f.batch = make([]string, 0, f.batchSize)
	f.inboxCh = make(chan string, f.batchSize)
	f.completedCh = make(chan interface{}, 1)

	f.includes, err = CompileGlobs(config.Includes)
	if err != nil {
		return nil, fmt.Errorf("%w: formatter '%v' includes", err, f.name)
	}

	f.excludes, err = CompileGlobs(config.Excludes)
	if err != nil {
		return nil, fmt.Errorf("%w: formatter '%v' excludes", err, f.name)
	}
	f.excludes = append(f.excludes, globalExcludes...)

	return &f, nil
}

func (f *Formatter) SetParent(formatter *Formatter) {
	f.parent = formatter
}

func (f *Formatter) Parent() *Formatter {
	return f.parent
}

func (f *Formatter) SetChild(formatter *Formatter) {
	f.child = formatter
}

// Wants is used to test if a Formatter wants path based on it's configured Includes and Excludes patterns.
// Returns true if the Formatter should be applied to path, false otherwise.
func (f *Formatter) Wants(path string) bool {
	if f.parent != nil {
		// we don't accept this path directly, our parent will forward it
		return false
	}
	match := !PathMatches(path, f.excludes) && PathMatches(path, f.includes)
	if match {
		f.log.Debugf("match: %v", path)
	}
	return match
}

// Put add path into this Formatter's inboxCh for processing.
func (f *Formatter) Put(path string) {
	f.inboxCh <- path
}

// Run is the main processing loop for this Formatter.
// It accepts a context which is used to lookup certain dependencies and for cancellation.
func (f *Formatter) Run(ctx context.Context) (err error) {
	defer func() {
		if f.child != nil {
			// indicate no further processing for the child formatter
			f.child.Close()
		}

		// indicate this formatter has finished processing
		f.completedCh <- nil
	}()

LOOP:
	// keep processing until ctx has been cancelled or inboxCh has been closed
	for {
		select {

		case <-ctx.Done():
			// ctx has been cancelled
			err = ctx.Err()
			break LOOP

		case path, ok := <-f.inboxCh:
			// check if the inboxCh has been closed
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
	args := f.config.Options

	// append each file path
	for _, path := range f.batch {
		args = append(args, path)
	}

	// execute
	start := time.Now()
	cmd := exec.CommandContext(ctx, f.config.Command, args...)

	if out, err := cmd.CombinedOutput(); err != nil {
		f.log.Debugf("\n%v", string(out))
		// todo log output
		return err
	}

	f.log.Infof("%v files processed in %v", len(f.batch), time.Now().Sub(start))

	if f.child == nil {
		// mark each path in this batch as completed
		for _, path := range f.batch {
			MarkPathComplete(ctx, path)
		}
	} else {
		// otherwise forward each path onto the next formatter for processing
		for _, path := range f.batch {
			f.child.Put(path)
		}
	}

	// reset batch
	f.batch = f.batch[:0]

	return nil
}

// Close is used to indicate that a Formatter should process any remaining paths and then stop it's processing loop.
func (f *Formatter) Close() {
	close(f.inboxCh)
}

func (f *Formatter) AwaitCompletion() {
	// todo support a timeout
	<-f.completedCh
}
