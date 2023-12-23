package format

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gobwas/glob"
	"github.com/juju/errors"
)

type Formatter struct {
	Name     string
	Command  string
	Options  []string
	Includes []string
	Excludes []string
	Before   []string

	log *log.Logger

	// globs for matching against paths
	includes []glob.Glob
	excludes []glob.Glob

	inbox chan string

	batch     []string
	batchSize int
}

func (f *Formatter) Init(name string) error {
	f.Name = name
	f.log = log.WithPrefix("format | " + name)

	f.inbox = make(chan string, 1024)

	f.batchSize = 1024
	f.batch = make([]string, f.batchSize)
	f.batch = f.batch[:0]

	// todo refactor common code below
	if len(f.Includes) > 0 {
		for _, pattern := range f.Includes {
			if !strings.Contains(pattern, "/") {
				pattern = "**/" + pattern
			}
			g, err := glob.Compile(pattern)
			if err != nil {
				return errors.Annotatef(err, "failed to compile include pattern '%v' for formatter '%v'", pattern, f.Name)
			}
			f.includes = append(f.includes, g)
		}
	}

	if len(f.Excludes) > 0 {
		for _, pattern := range f.Excludes {
			if !strings.Contains(pattern, "/") {
				pattern = "**/" + pattern
			}
			g, err := glob.Compile(pattern)
			if err != nil {
				return errors.Annotatef(err, "failed to compile exclude pattern '%v' for formatter '%v'", pattern, f.Name)
			}
			f.excludes = append(f.excludes, g)
		}
	}

	return nil
}

func (f *Formatter) Wants(path string) bool {
	if PathMatches(path, f.excludes) {
		return false
	}
	return PathMatches(path, f.includes)
}

func (f *Formatter) Put(path string) {
	f.inbox <- path
}

func (f *Formatter) Run(ctx context.Context) (err error) {
LOOP:
	for {
		select {
		case <-ctx.Done():
			err = ctx.Err()
			break LOOP

		case path, ok := <-f.inbox:
			if !ok {
				break LOOP
			}

			// add to the current batch
			f.batch = append(f.batch, path)

			if len(f.batch) == f.batchSize {
				// drain immediately
				if err := f.apply(ctx); err != nil {
					break LOOP
				}
			}
		}
	}

	if err != nil {
		return
	}

	// final flush
	return f.apply(ctx)
}

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

	start := time.Now()
	cmd := exec.CommandContext(ctx, f.Command, args...)

	if _, err := cmd.CombinedOutput(); err != nil {
		// todo log output
		return err
	}

	f.log.Infof("%v files processed in %v", len(f.batch), time.Now().Sub(start))

	// mark completed or forward on
	if len(f.Before) == 0 {
		for _, path := range f.batch {
			MarkFormatComplete(ctx, path)
		}
	} else {
		for _, path := range f.batch {
			ForwardPath(ctx, path, f.Before)
		}
	}

	// reset batch
	f.batch = f.batch[:0]

	return nil
}

func (f *Formatter) Close() {
	close(f.inbox)
}
