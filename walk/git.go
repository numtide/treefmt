package walk

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/format/index"

	"github.com/go-git/go-git/v5"
)

// fileTree represents a hierarchical file structure with directories and files.
type fileTree struct {
	name    string
	entries map[string]*fileTree
}

// add inserts a file path into the fileTree structure, creating necessary parent directories if they do not exist.
func (n *fileTree) add(path []string) {
	if len(path) == 0 {
		return
	} else if n.entries == nil {
		n.entries = make(map[string]*fileTree)
	}

	name := path[0]
	child, ok := n.entries[name]
	if !ok {
		child = &fileTree{name: name}
		n.entries[name] = child
	}
	child.add(path[1:])
}

// addPath splits the given path by the filepath separator and inserts it into the fileTree structure.
func (n *fileTree) addPath(path string) {
	n.add(strings.Split(path, string(filepath.Separator)))
}

// has returns true if the specified path exists in the fileTree, false otherwise.
func (n *fileTree) has(path []string) bool {
	if len(path) == 0 {
		return true
	} else if len(n.entries) == 0 {
		return false
	}
	child, ok := n.entries[path[0]]
	if !ok {
		return false
	}
	return child.has(path[1:])
}

// hasPath splits the given path by the filepath separator and checks if it exists in the fileTree.
func (n *fileTree) hasPath(path string) bool {
	return n.has(strings.Split(path, string(filepath.Separator)))
}

// readIndex traverses the index entries and adds each file path to the fileTree structure.
func (n *fileTree) readIndex(idx *index.Index) {
	for _, entry := range idx.Entries {
		n.addPath(entry.Name)
	}
}

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

func (g gitWalker) relPath(path string) (string, error) { //
	return filepath.Rel(g.root, path)
}

func (g gitWalker) Walk(ctx context.Context, fn WalkFunc) error {
	idx, err := g.repo.Storer.Index()
	if err != nil {
		return fmt.Errorf("failed to open git index: %w", err)
	}

	// if we need to walk a path that is not the root of the repository, we will read the directory structure of the
	// git index into memory for faster lookups
	var cache *fileTree

	for path := range g.paths {
		switch path {

		case g.root:

			// we can just iterate the index entries
			for _, entry := range idx.Entries {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
					// we only want regular files, not directories or symlinks
					if entry.Mode == filemode.Dir || entry.Mode == filemode.Symlink {
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

		default:

			// read the git index into memory if it hasn't already
			if cache == nil {
				cache = &fileTree{name: ""}
				cache.readIndex(idx)
			}

			// git index entries are relative to the repository root, so we need to determine a relative path for the
			// one we are currently processing before checking if it exists within the git index
			relPath, err := g.relPath(path)
			if err != nil {
				return fmt.Errorf("failed to find root relative path for %v: %w", path, err)
			}

			if !cache.hasPath(relPath) {
				log.Debugf("path %s not found in git index, skipping", relPath)
				continue
			}

			err = filepath.Walk(path, func(path string, info fs.FileInfo, _ error) error {
				// skip directories
				if info.IsDir() {
					return nil
				}

				// determine a path relative to g.root before checking presence in the git index
				relPath, err := g.relPath(path)
				if err != nil {
					return fmt.Errorf("failed to determine a relative path for %s: %w", path, err)
				}

				if !cache.hasPath(relPath) {
					log.Debugf("path %v not found in git index, skipping", relPath)
					return nil
				}

				file := File{
					Path:    path,
					RelPath: relPath,
					Info:    info,
				}

				return fn(&file, err)
			})
			if err != nil {
				return fmt.Errorf("failed to walk %s: %w", path, err)
			}
		}
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
