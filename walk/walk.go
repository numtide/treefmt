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
)

type File struct {
	Path    string
	RelPath string
	Info    fs.FileInfo

	CacheEntry *cache.Entry

	Release func()
}

func (f File) Stat() (bool, fs.FileInfo, error) {
	// get the file's current state
	current, err := os.Stat(f.Path)
	if err != nil {
		return false, nil, fmt.Errorf("failed to stat %s: %w", f.Path, err)
	}

	// check the size first
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

func (f File) String() string {
	return f.Path
}

type Reader interface {
	Read(ctx context.Context, files []*File) (n int, err error)
	Close() error
}

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
		reader, err = NewGitReader(root, paths, statz, 1024)
	case Filesystem:
		reader = NewFilesystemReader(root, paths, statz, 1024)
	default:
		return nil, fmt.Errorf("unknown walk type: %v", walkType)
	}

	if err != nil {
		return nil, err
	}

	// wrap with cached reader
	reader, err = NewCachedReader(db, 1024, reader)

	return reader, err
}
