package walker

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"time"
)

type Type string

const (
	Git        Type = "git"
	Auto       Type = "auto"
	Filesystem Type = "filesystem"
)

type File struct {
	Path    string
	RelPath string
	Info    fs.FileInfo
}

func (f File) HasChanged() (bool, fs.FileInfo, error) {
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
	// Some formatters mess with the mod time (e.g., dos2unix) but not to the same precision,
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

type WalkFunc func(file *File, err error) error

type Walker interface {
	Root() string
	Walk(ctx context.Context, fn WalkFunc) error
}

func New(walkerType Type, root string, noCache bool, pathsCh chan string) (Walker, error) {
	switch walkerType {
	case Git:
		return NewGit(root, noCache, pathsCh)
	case Auto:
		return Detect(root, noCache, pathsCh)
	case Filesystem:
		return NewFilesystem(root, pathsCh)
	default:
		return nil, fmt.Errorf("unknown walker type: %v", walkerType)
	}
}

func Detect(root string, noCache bool, pathsCh chan string) (Walker, error) {
	// for now, we keep it simple and try git first, filesystem second
	w, err := NewGit(root, noCache, pathsCh)
	if err == nil {
		return w, err
	}
	return NewFilesystem(root, pathsCh)
}
