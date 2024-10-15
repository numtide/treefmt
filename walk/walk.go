package walk

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/numtide/treefmt/stats"
	"github.com/numtide/treefmt/walk/cache"
	bolt "go.etcd.io/bbolt"
)

//go:generate enumer -type=Type -text -transform=snake -output=./type_enum.go
type Type int

const (
	Auto Type = iota
	Git
	Filesystem
	Stdin

	BatchSize = 1024
)

type ReleaseFunc func(formatErr error) error

// File represents a file object with its path, relative path, file info, and potential cache entry.
type File struct {
	Path    string
	RelPath string
	Info    fs.FileInfo

	// Cache is the latest entry found for this file, if one exists.
	Cache *cache.Entry

	releaseFuncs []ReleaseFunc
}

// Release calls all registered release functions for the File and returns an error if any function fails.
// Accepts formatErr, which indicates if an error occurred when formatting this file.
func (f *File) Release(formatErr error) error {
	for _, fn := range f.releaseFuncs {
		if err := fn(formatErr); err != nil {
			return err
		}
	}
	return nil
}

// AddReleaseFunc adds a release function to the File's list of release functions.
func (f *File) AddReleaseFunc(fn ReleaseFunc) {
	f.releaseFuncs = append(f.releaseFuncs, fn)
}

// Stat checks if the file has changed by comparing its current state (size, mod time) to when it was first read.
// It returns a boolean indicating if the file has changed, the current file info, and an error if any.
func (f *File) Stat() (changed bool, info fs.FileInfo, err error) {
	// Get the file's current state
	current, err := os.Stat(f.Path)
	if err != nil {
		return false, nil, fmt.Errorf("failed to stat %s: %w", f.Path, err)
	}

	// Check the size first
	if f.Info.Size() != current.Size() {
		return true, current, nil
	}

	// POSIX specifies EPOCH time for Mod time, but some filesystems give more precision.
	// Some formatters mess with the mod time (e.g. dos2unix) but not to the same precision,
	// triggering false positives.
	// We truncate everything below a second.
	if f.Info.ModTime().Truncate(time.Second) != current.ModTime().Truncate(time.Second) {
		return true, current, nil
	}

	return false, nil, nil
}

// String returns the file's path as a string.
func (f *File) String() string {
	return f.Path
}

// Reader is an interface for reading files.
type Reader interface {
	Read(ctx context.Context, files []*File) (n int, err error)
	Close() error
}

// CompositeReader combines multiple Readers into one.
// It iterates over the given readers, reading each until completion.
type CompositeReader struct {
	idx     int
	current Reader
	readers []Reader
}

func (c *CompositeReader) Read(ctx context.Context, files []*File) (n int, err error) {
	if c.current == nil {
		// check if we have exhausted all the readers
		if c.idx >= len(c.readers) {
			return 0, io.EOF
		}

		// if not, select the next reader
		c.current = c.readers[c.idx]
		c.idx++
	}

	// attempt a read
	n, err = c.current.Read(ctx, files)

	// check if the current reader has been exhausted
	if errors.Is(err, io.EOF) {
		// reset the error if it's EOF
		err = nil
		// set the current reader to nil so we try to read from the next reader on the next call
		c.current = nil
	} else if err != nil {
		err = fmt.Errorf("failed to read from current reader: %w", err)
	}

	// return the number of files read in this call and any error
	return n, err
}

func (c *CompositeReader) Close() error {
	for _, reader := range c.readers {
		if err := reader.Close(); err != nil {
			return fmt.Errorf("failed to close reader: %w", err)
		}
	}
	return nil
}

func NewReader(
	walkType Type,
	root string,
	path string,
	db *bolt.DB,
	statz *stats.Stats,
) (Reader, error) {
	var (
		err    error
		reader Reader
	)

	switch walkType {
	case Auto:
		// for now, we keep it simple and try git first, filesystem second
		reader, err = NewReader(Git, root, path, db, statz)
		if err != nil {
			reader, err = NewReader(Filesystem, root, path, db, statz)
		}
		return reader, err
	case Git:
		reader, err = NewGitReader(root, path, statz, BatchSize)
	case Filesystem:
		reader = NewFilesystemReader(root, path, statz, BatchSize)
	case Stdin:
		return nil, fmt.Errorf("stdin walk type is not supported")
	default:
		return nil, fmt.Errorf("unknown walk type: %v", walkType)
	}

	if err != nil {
		return nil, err
	}

	if db != nil {
		// wrap with cached reader
		// db will be null if --no-cache is enabled
		reader, err = NewCachedReader(db, BatchSize, reader)
	}

	return reader, err
}

func NewCompositeReader(
	walkType Type,
	root string,
	paths []string,
	db *bolt.DB,
	statz *stats.Stats,
) (Reader, error) {
	// if not paths are provided we default to processing the tree root
	if len(paths) == 0 {
		return NewReader(walkType, root, "", db, statz)
	}

	readers := make([]Reader, len(paths))

	// check we have received 1 path for the stdin walk type
	if walkType == Stdin {
		if len(paths) != 1 {
			return nil, fmt.Errorf("stdin walk requires exactly one path")
		}

		return NewStdinReader(root, paths[0], statz), nil
	}

	// create a reader for each provided path
	for idx, relPath := range paths {
		var (
			err  error
			info os.FileInfo
		)

		// create a clean absolute path
		path := filepath.Clean(filepath.Join(root, relPath))

		// check the path exists
		info, err = os.Lstat(path)
		if err != nil {
			return nil, fmt.Errorf("failed to stat %s: %w", path, err)
		}

		if info.IsDir() {
			// for directories, we honour the walk type as we traverse them
			readers[idx], err = NewReader(walkType, root, relPath, db, statz)
		} else {
			// for files, we enforce a simple filesystem read
			readers[idx], err = NewReader(Filesystem, root, relPath, db, statz)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to create reader for %s: %w", relPath, err)
		}
	}

	return &CompositeReader{
		readers: readers,
	}, nil
}
