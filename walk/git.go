package walk

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"

	"github.com/go-git/go-git/v5"
)

type gitWalker struct {
	root  string
	paths chan string
	repo  *git.Repository
}

func (g *gitWalker) Root() string {
	return g.root
}

func (g *gitWalker) Walk(ctx context.Context, fn WalkFunc) error {
	// for quick relative paths
	relPathOffset := len(g.root) + 1

	relPathFn := func(path string) (relPath string) {
		if len(path) >= relPathOffset {
			relPath = path[relPathOffset:]
		}
		return
	}

	idx, err := g.repo.Storer.Index()
	if err != nil {
		return fmt.Errorf("failed to open git index: %w", err)
	}

	// cache in-memory whether a path is present in the git index
	var cache map[string]bool

	for path := range g.paths {

		if path == g.root {
			// we can just iterate the index entries
			for _, entry := range idx.Entries {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
					path := filepath.Join(g.root, entry.Name)

					// stat the file
					info, err := os.Lstat(path)

					file := File{
						Path:    path,
						RelPath: relPathFn(path),
						Info:    info,
					}

					if err = fn(&file, err); err != nil {
						return err
					}
				}
			}
			continue
		}

		// otherwise we ensure the git index entries are cached and then check if they are in the git index
		if cache == nil {
			cache = make(map[string]bool)
			for _, entry := range idx.Entries {
				cache[entry.Name] = true
			}
		}

		relPath, err := filepath.Rel(g.root, path)
		if err != nil {
			return fmt.Errorf("failed to find relative path for %v: %w", path, err)
		}

		_, ok := cache[relPath]
		if !(path == g.root || ok) {
			log.Debugf("path %v not found in git index, skipping", path)
			continue
		}

		return filepath.Walk(path, func(path string, info fs.FileInfo, err error) error {
			if info.IsDir() {
				return nil
			}

			relPath, err := filepath.Rel(g.root, path)
			if err != nil {
				return err
			}

			if _, ok := cache[relPath]; !ok {
				log.Debugf("path %v not found in git index, skipping", path)
				return nil
			}

			file := File{
				Path:    path,
				RelPath: relPathFn(path),
				Info:    info,
			}

			return fn(&file, err)
		})
	}

	return nil
}

func NewGit(root string, paths chan string) (Walker, error) {
	repo, err := git.PlainOpen(root)
	if err != nil {
		return nil, fmt.Errorf("failed to open git repo: %w", err)
	}
	return &gitWalker{root, paths, repo}, nil
}
