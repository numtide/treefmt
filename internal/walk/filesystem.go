package walk

import (
	"context"
	"path/filepath"
)

type filesystem struct {
	root string
}

func (f filesystem) Root() string {
	return f.root
}

func (f filesystem) Walk(_ context.Context, fn filepath.WalkFunc) error {
	return filepath.Walk(f.root, fn)
}

func NewFilesystem(root string) (Walker, error) {
	return filesystem{root}, nil
}
