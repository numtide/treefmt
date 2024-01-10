package walk

import (
	"context"
	"fmt"
	"path/filepath"
)

type Type string

const (
	Git        Type = "git"
	Auto       Type = "auto"
	Filesystem Type = "filesystem"
)

type Walker interface {
	Root() string
	Walk(ctx context.Context, fn filepath.WalkFunc) error
}

func New(walkerType Type, root string, paths []string) (Walker, error) {
	switch walkerType {
	case Git:
		return NewGit(root, paths)
	case Auto:
		return Detect(root, paths)
	case Filesystem:
		return NewFilesystem(root, paths)
	default:
		return nil, fmt.Errorf("unknown walker type: %v", walkerType)
	}
}

func Detect(root string, paths []string) (Walker, error) {
	// for now, we keep it simple and try git first, filesystem second
	w, err := NewGit(root, paths)
	if err == nil {
		return w, err
	}
	return NewFilesystem(root, paths)
}
