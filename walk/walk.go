package walk

import (
	"context"
	"fmt"
	"io/fs"
	"os"
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

	BatchSize = 1024
)

// File represents a file object with its path, relative path, file info, and potential cached entry.
// It provides an optional release function to trigger a cache update after processing.
type File struct {
	Path    string
	RelPath string
	Info    fs.FileInfo

	// Cache is the latest entry found for this file, if one exists.
	Cache *cache.Entry

	// An optional function to be invoked when this File has finished processing.
	// Typically used to trigger a cache update.
	Release func()
}

// Stat checks if the file has changed by comparing its current state (size, mod time) to when it was first read.
// It returns a boolean indicating if the file has changed, the current file info, and an error if any.
func (f File) Stat() (bool, fs.FileInfo, error) {
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
func (f File) String() string {
	return f.Path
}

// Reader is an interface for reading files.
type Reader interface {
	Read(ctx context.Context, files []*File) (n int, err error)
	Close() error
}

// NewReader creates a new instance of Reader based on the given walkType (Auto, Git, Filesystem).
func NewReader(
	walkType Type,
	root string,
	paths []string,
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
		reader, err = NewReader(Git, root, paths, db, statz)
		if err != nil {
			reader, err = NewReader(Filesystem, root, paths, db, statz)
		}
		return reader, err
	case Git:
		reader, err = NewGitReader(root, paths, statz, BatchSize)
	case Filesystem:
		reader = NewFilesystemReader(root, paths, statz, BatchSize)
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
