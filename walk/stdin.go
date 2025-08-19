package walk

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/numtide/treefmt/v2/stats"
)

type StdinReader struct {
	root  string
	path  string
	stats stats.Stats
	input *os.File

	complete bool
}

func (s StdinReader) Read(_ context.Context, files []*File) (n int, err error) {
	if s.complete {
		return 0, io.EOF
	}

	// read stdin into a temporary file with the same file extension
	pattern := "*" + filepath.Ext(s.path)

	file, err := os.CreateTemp(s.root, pattern)
	if err != nil {
		return 0, fmt.Errorf("failed to create a temporary file for processing stdin: %w", err)
	}
	defer file.Close()

	if _, err = io.Copy(file, s.input); err != nil {
		return 0, errors.New("failed to copy stdin into a temporary file")
	}

	info, err := file.Stat()
	if err != nil {
		return 0, fmt.Errorf("failed to get file info for temporary file: %w", err)
	}

	relPath, err := filepath.Rel(s.root, s.path)
	if err != nil {
		return 0, fmt.Errorf("failed to get relative path for file: %w", err)
	}

	files[0] = &File{
		Path:         s.path,
		RelPath:      relPath,
		Info:         info,
		PathToFormat: file.Name(),
	}

	// dump the temp file to stdout and remove it once the file is finished being processed
	files[0].AddReleaseFunc(func(_ context.Context) error {
		// open the temp file
		file, err := os.Open(file.Name())
		if err != nil {
			return fmt.Errorf("failed to open temp file %s: %w", file.Name(), err)
		}

		// dump file into stdout
		if _, err = io.Copy(os.Stdout, file); err != nil {
			return fmt.Errorf("failed to copy %s to stdout: %w", file.Name(), err)
		}

		if err = file.Close(); err != nil {
			return fmt.Errorf("failed to close temp file %s: %w", file.Name(), err)
		}

		// clean up the temp file
		if err = os.Remove(file.Name()); err != nil {
			return fmt.Errorf("failed to remove temp file %s: %w", file.Name(), err)
		}

		return nil
	})

	s.complete = true
	s.stats.Add(stats.Traversed, 1)

	return 1, io.EOF
}

func (s StdinReader) Close() error {
	return nil
}

func NewStdinReader(root string, path string, statz *stats.Stats) StdinReader {
	return StdinReader{
		root:  root,
		path:  path,
		stats: *statz,
		input: os.Stdin,
	}
}
