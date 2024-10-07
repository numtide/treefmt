package walk

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

type FilesystemWalker struct {
	root          string
	pathsCh       chan string
	relPathOffset int
}

func (f FilesystemWalker) Root() string {
	return f.root
}

func (f FilesystemWalker) relPath(path string) (string, error) {
	return filepath.Rel(f.root, path)
}

func (f FilesystemWalker) Walk(_ context.Context, fn WalkerFunc) error {
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

func NewFilesystem(root string, paths chan string) (*FilesystemWalker, error) {
	return &FilesystemWalker{
		root:          root,
		pathsCh:       paths,
		relPathOffset: len(root) + 1,
	}, nil
}
