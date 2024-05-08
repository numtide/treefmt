package walk

import (
	"context"
	"fmt"
	"io/fs"
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

func (f File) String() string {
	return f.Path
}

type WalkFunc func(file *File, err error) error

type Walker interface {
	Root() string
	Walk(ctx context.Context, fn WalkFunc) error
}

func New(walkerType Type, root string, pathsCh chan string) (Walker, error) {
	switch walkerType {
	case Git:
		return NewGit(root, pathsCh)
	case Auto:
		return Detect(root, pathsCh)
	case Filesystem:
		return NewFilesystem(root, pathsCh)
	default:
		return nil, fmt.Errorf("unknown walker type: %v", walkerType)
	}
}

func Detect(root string, pathsCh chan string) (Walker, error) {
	// for now, we keep it simple and try git first, filesystem second
	w, err := NewGit(root, pathsCh)
	if err == nil {
		return w, err
	}
	return NewFilesystem(root, pathsCh)
}
