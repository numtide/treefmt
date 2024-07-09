package walker

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

type filesystemWalker struct {
	root          string
	pathsCh       chan string
	relPathOffset int
}

func (f filesystemWalker) Root() string {
	return f.root
}

func (f filesystemWalker) relPath(path string) (string, error) {
	// quick optimization for the majority of use cases
	if len(path) >= f.relPathOffset && path[:len(f.root)] == f.root {
		return path[f.relPathOffset:], nil
	}
	// fallback to proper relative path resolution
	return filepath.Rel(f.root, path)
}

func (f filesystemWalker) Walk(_ context.Context, fn WalkFunc) error {
	walkFn := func(path string, info fs.FileInfo, _ error) error {
		if info == nil {
			return fmt.Errorf("no such file or directory '%s'", path)
		}

		// ignore directories and symlinks
		if info.IsDir() || info.Mode()&os.ModeSymlink == os.ModeSymlink {
			return nil
		}

		relPath, err := f.relPath(path)
		if err != nil {
			return fmt.Errorf("failed to determine a relative path for %s: %w", path, err)
		}

		file := File{
			Path:    path,
			RelPath: relPath,
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
	return filesystemWalker{
		root:          root,
		pathsCh:       paths,
		relPathOffset: len(root) + 1,
	}, nil
}
