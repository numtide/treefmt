package walk

import (
	"context"
	"io/fs"
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

	if len(f.paths) == 0 {
		return filepath.Walk(f.root, walkFn)
	}

	for _, path := range f.paths {
		info, err := os.Stat(path)
		if err = filepath.Walk(path, walkFn); err != nil {
			return err
		}

		file := File{
			Path:    path,
			RelPath: relPathFn(path),
			Info:    info,
		}

		if err = fn(&file, err); err != nil {
			return err
		}
	}

	return nil
}

func NewFilesystem(root string, paths []string) (Walker, error) {
	return filesystemWalker{root, paths}, nil
}
