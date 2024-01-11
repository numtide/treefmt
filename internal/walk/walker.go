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

func New(walkerType Type, root string) (Walker, error) {
	switch walkerType {
	case Git:
		return NewGit(root)
	case Auto:
		return Detect(root)
	case Filesystem:
		return NewFilesystem(root)
	default:
		return nil, fmt.Errorf("unknown walker type: %v", walkerType)
	}
}

func Detect(root string) (Walker, error) {
	// for now, we keep it simple and try git first, filesystem second
	w, err := NewGit(root)
	if err == nil {
		return w, err
	}
	return NewFilesystem(root)
}
