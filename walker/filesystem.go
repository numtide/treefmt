package walker

import (
	"context"
	"fmt"
	"git.numtide.com/numtide/treefmt/cache"
	"git.numtide.com/numtide/treefmt/stats"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

type filesystemWalker struct {
	root          string
	cache         *cache.Cache
	pathsCh       chan string
	relPathOffset int
}

func (f filesystemWalker) UpdatePaths(batch []*File) error {
	if err := f.cache.Update(func(tx *cache.Tx) error {
		// get the paths bucket
		paths, err := tx.Paths()
		if err != nil {
			return err
		}
		for _, f := range batch {
			entry := cache.Entry{
				Size:     f.Info.Size(),
				Modified: f.Info.ModTime(),
			}
			if err := paths.Put(f.RelPath, &entry); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		return fmt.Errorf("failed to update paths: %w", err)
	}
	return nil
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

	var tx *cache.Tx
	var paths *cache.Bucket[cache.Entry]
	var processed int
	batchSize := 1024 * runtime.NumCPU()

	defer func() {
		// close any pending read tx
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

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

		// open a new read tx if there isn't one in progress
		// we have to periodically open a new read tx to prevent writes from being blocked
		if tx == nil {
			if tx, err = f.cache.BeginTx(false); err != nil {
				return fmt.Errorf("failed to open a new cache read tx: %w", err)
			} else if paths, err = tx.Paths(); err != nil {
				return fmt.Errorf("failed to get paths bucket from cache tx: %w", err)
			}
		}

		cached, err := paths.Get(file.RelPath)
		if err != nil {
			return err
		}

		// close the current tx if we have reached the batch size
		processed += 1
		if processed == batchSize {
			if err = tx.Rollback(); err != nil {
				return err
			}
			tx = nil
		}

		//
		changedOrNew := cached == nil || !(cached.Modified == file.Info.ModTime() && cached.Size == file.Info.Size())

		stats.Add(stats.Traversed, 1)
		if !changedOrNew {
			// no change
			return nil
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

func NewFilesystem(root string, cache *cache.Cache, paths chan string) (Walker, error) {
	return filesystemWalker{
		root:          root,
		cache:         cache,
		pathsCh:       paths,
		relPathOffset: len(root) + 1,
	}, nil
}
