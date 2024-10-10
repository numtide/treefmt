package walk

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/charmbracelet/log"
	"github.com/go-git/go-git/v5"
	"github.com/numtide/treefmt/stats"
	"golang.org/x/sync/errgroup"
)

type GitReader struct {
	root      string
	paths     []string
	stats     *stats.Stats
	batchSize int

	log  *log.Logger
	repo *git.Repository

	filesCh chan *File

	eg *errgroup.Group
}

func (g *GitReader) process() error {
	defer func() {
		close(g.filesCh)
	}()

	r, w := io.Pipe()

	g.eg.Go(func() error {
		cmd := exec.Command("git", "status", "--short")
		cmd.Dir = g.root
		cmd.Stdout = w

		err := cmd.Run()
		w.CloseWithError(err)
		return err
	})

	tree := &filetree{name: ""}
	tree.fromGit(r)

	for _, subPath := range g.paths {

		subTree, ok := tree.getPath(subPath)
		if !ok {
			return fmt.Errorf("path %s not found in git status", subPath)
		}

		err := subTree.walk("", func(relPath string) error {
			// stat the file
			fullPath := filepath.Join(g.root, relPath)

			if fullPath == g.root {
				// skip the root
				return nil
			}

			info, err := os.Lstat(fullPath)
			if os.IsNotExist(err) {
				// the underlying file might have been removed without the change being staged yet
				g.log.Warnf("Path %s is in the index but appears to have been removed from the filesystem", fullPath)
				return nil
			} else if err != nil {
				return fmt.Errorf("failed to stat %s: %w", fullPath, err)
			}

			file := File{
				Path:    fullPath,
				RelPath: relPath,
				Info:    info,
			}

			g.stats.Add(stats.Traversed, 1)
			g.filesCh <- &file

			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to walk %s: %w", subPath, err)
		}
	}

	return nil
}

func (g *GitReader) Read(ctx context.Context, files []*File) (n int, err error) {
	idx := 0

LOOP:
	for idx < len(files) {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case file, ok := <-g.filesCh:
			if !ok {
				break LOOP
			}
			files[idx] = file
			idx++
		}
	}

	return idx, nil
}

func (g *GitReader) Close() error {
	return g.eg.Wait()
}

func NewGitReader(
	root string,
	paths []string,
	statz *stats.Stats,
	batchSize int,
) (*GitReader, error) {
	repo, err := git.PlainOpen(root)
	if err != nil {
		return nil, fmt.Errorf("failed to open git repository: %w", err)
	}

	eg := &errgroup.Group{}

	if len(paths) == 0 {
		paths = []string{"."}
	}

	r := &GitReader{
		root:      root,
		paths:     paths,
		stats:     statz,
		batchSize: batchSize,
		log:       log.WithPrefix("walk[git]"),
		repo:      repo,
		filesCh:   make(chan *File, batchSize*runtime.NumCPU()),
		eg:        eg,
	}

	eg.Go(r.process)

	return r, nil
}
