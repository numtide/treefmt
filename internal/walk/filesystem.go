package walk

import (
	"context"
	"os"
	"path/filepath"
)

type filesystemWalker struct {
	root  string
	paths []string
}

func (f filesystemWalker) Root() string {
	return f.root
}

func (f filesystemWalker) Walk(_ context.Context, fn filepath.WalkFunc) error {
	if len(f.paths) == 0 {
		return filepath.Walk(f.root, fn)
	}

	for _, path := range f.paths {
		info, err := os.Stat(path)
		if err = filepath.Walk(path, fn); err != nil {
			return err
		}

		if err = fn(path, info, err); err != nil {
			return err
		}
	}

	return nil
}

func NewFilesystem(root string, paths []string) (Walker, error) {
	return filesystemWalker{root, paths}, nil
}
