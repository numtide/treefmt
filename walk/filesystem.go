package walk

import (
	"context"
	"io/fs"
	"path/filepath"
)

type filesystemWalker struct {
	root    string
	pathsCh chan string
}

func (f filesystemWalker) Root() string {
	return f.root
}

func (f filesystemWalker) Walk(_ context.Context, fn WalkFunc) error {
	relPathOffset := len(f.root) + 1

	relPathFn := func(path string) (relPath string) {
		if len(path) >= relPathOffset {
			relPath = path[relPathOffset:]
		}
		return
	}

	walkFn := func(path string, info fs.FileInfo, err error) error {
		file := File{
			Path:    path,
			RelPath: relPathFn(path),
			Info:    info,
		}
		return fn(&file, err)
	}

	for path := range f.pathsCh {
		if err := filepath.Walk(path, walkFn); err != nil {
			return err
		}
	}

	return nil
}

func NewFilesystem(root string, paths chan string) (Walker, error) {
	return filesystemWalker{root, paths}, nil
}
