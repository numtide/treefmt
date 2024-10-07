package walk

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/format/index"
)

// FileTree represents a hierarchical file structure with directories and files.
type FileTree struct {
	Name    string
	Entries map[string]*FileTree
}

// Add inserts a file path into the FileTree structure, creating necessary parent directories if they do not exist.
func (n *FileTree) Add(path []string) {
	if len(path) == 0 {
		return
	} else if n.Entries == nil {
		n.Entries = make(map[string]*FileTree)
	}

	name := path[0]

	child, ok := n.Entries[name]
	if !ok {
		child = &FileTree{Name: name}
		n.Entries[name] = child
	}

	child.Add(path[1:])
}

// AddPath splits the given path by the filepath separator and inserts it into the FileTree structure.
func (n *FileTree) AddPath(path string) {
	n.Add(strings.Split(path, string(filepath.Separator)))
}

// Has returns true if the specified path exists in the FileTree, false otherwise.
func (n *FileTree) Has(path []string) bool {
	if len(path) == 0 {
		return true
	} else if len(n.Entries) == 0 {
		return false
	}

	child, ok := n.Entries[path[0]]
	if !ok {
		return false
	}

	return child.Has(path[1:])
}

// HasPath splits the given path by the filepath separator and checks if it exists in the FileTree.
func (n *FileTree) HasPath(path string) bool {
	return n.Has(strings.Split(path, string(filepath.Separator)))
}

// ReadIndex traverses the index entries and adds each file path to the FileTree structure.
func (n *FileTree) ReadIndex(idx *index.Index) {
	for _, entry := range idx.Entries {
		n.AddPath(entry.Name)
	}
}

type GitWalker struct {
	log           *log.Logger
	root          string
	paths         chan string
	repo          *git.Repository
	relPathOffset int
}

func (g GitWalker) Root() string {
	return g.root
}

func (g GitWalker) relPath(path string) (string, error) { //
	return filepath.Rel(g.root, path)
}

func (g GitWalker) Walk(ctx context.Context, fn WalkerFunc) error {
	idx, err := g.repo.Storer.Index()
	if err != nil {
		return fmt.Errorf("failed to open git index: %w", err)
	}

	// if we need to walk a path that is not the root of the repository, we will read the directory structure of the
	// git index into memory for faster lookups
	var cache *FileTree

	for path := range g.paths {
		switch path {
		case g.root:
			// we can just iterate the index Entries
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
				cache = &FileTree{Name: ""}
				cache.ReadIndex(idx)
			}

			// git index Entries are relative to the repository root, so we need to determine a relative path for the
			// one we are currently processing before checking if it exists within the git index
			relPath, err := g.relPath(path)
			if err != nil {
				return fmt.Errorf("failed to find root relative path for %v: %w", path, err)
			}

			if !cache.HasPath(relPath) {
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

				if !cache.HasPath(relPath) {
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

func NewGit(root string, paths chan string) (*GitWalker, error) {
	repo, err := git.PlainOpen(root)
	if err != nil {
		return nil, fmt.Errorf("failed to open git repo: %w", err)
	}

	return &GitWalker{
		log:           log.WithPrefix("walker[git]"),
		root:          root,
		paths:         paths,
		repo:          repo,
		relPathOffset: len(root) + 1,
	}, nil
}
