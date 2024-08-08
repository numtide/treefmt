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
	log           *log.Logger
	root          string
	paths         chan string
	repo          *git.Repository
	relPathOffset int
}

func (g gitWalker) Root() string {
	return g.root
}

func (g gitWalker) relPath(path string) (string, error) {
	// quick optimization for the majority of use cases
	if len(path) >= g.relPathOffset && path[:len(g.root)] == g.root {
		return path[g.relPathOffset:], nil
	}
	// fallback to proper relative path resolution
	return filepath.Rel(g.root, path)
}

func (g gitWalker) Walk(ctx context.Context, fn WalkFunc) error {
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
					// we only want regular files, not directories or symlinks
					if !entry.Mode.IsRegular() {
						continue
					}

					// stat the file
					path := filepath.Join(g.root, entry.Name)

					info, err := os.Lstat(path)
					if os.IsNotExist(err) {
						// the underlying file might have been removed without the change being staged yet
						g.log.Warnf("Path %s is in the index but appears to have been removed from the filesystem", path)
						continue
					} else if err != nil {
						return fmt.Errorf("failed to stat %s: %w", path, err)
					}

					// determine a relative path
					relPath, err := g.relPath(path)
					if err != nil {
						return fmt.Errorf("failed to determine a relative path for %s: %w", path, err)
					}

					file := File{
						Path:    path,
						RelPath: relPath,
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

		return filepath.Walk(path, func(path string, info fs.FileInfo, _ error) error {
			if info.IsDir() {
				return nil
			}

			relPath, err := g.relPath(path)
			if err != nil {
				return fmt.Errorf("failed to determine a relative path for %s: %w", path, err)
			}

			if _, ok := cache[relPath]; !ok {
				log.Debugf("path %v not found in git index, skipping", path)
				return nil
			}

			file := File{
				Path:    path,
				RelPath: relPath,
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
	return &gitWalker{
		log:           log.WithPrefix("walker[git]"),
		root:          root,
		paths:         paths,
		repo:          repo,
		relPathOffset: len(root) + 1,
	}, nil
}
