package walk

import (
	"context"
	"path/filepath"
)

type filesystemWalker struct {
	root string
}

func (f filesystemWalker) Root() string {
	return f.root
}

func (f filesystemWalker) Walk(_ context.Context, fn filepath.WalkFunc) error {
	return filepath.Walk(f.root, fn)
}

func NewFilesystem(root string) (Walker, error) {
	return filesystemWalker{root}, nil
}
