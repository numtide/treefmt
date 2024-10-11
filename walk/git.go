package walk

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/numtide/treefmt/stats"
	"golang.org/x/sync/errgroup"
)

type GitReader struct {
	root      string
	path      string
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

	gitIndex, err := g.repo.Storer.Index()
	if err != nil {
		return fmt.Errorf("failed to open git index: %w", err)
	}

	// if we need to walk a path that is not the root of the repository, we will read the directory structure of the
	// git index into memory for faster lookups
	var idxCache *filetree

	path := filepath.Clean(filepath.Join(g.root, g.path))
	if !strings.HasPrefix(path, g.root) {
		return fmt.Errorf("path '%s' is outside of the root '%s'", path, g.root)
	}

	switch path {

	case g.root:

		// we can just iterate the index entries
		for _, entry := range gitIndex.Entries {

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
			relPath, err := filepath.Rel(g.root, path)
			if err != nil {
				return fmt.Errorf("failed to determine a relative path for %s: %w", path, err)
			}

			file := File{
				Path:    path,
				RelPath: relPath,
				Info:    info,
			}

			g.stats.Add(stats.Traversed, 1)
			g.filesCh <- &file
		}

	default:

		// read the git index into memory if it hasn't already
		if idxCache == nil {
			idxCache = &filetree{name: ""}
			idxCache.readIndex(gitIndex)
		}

		// git index entries are relative to the repository root, so we need to determine a relative path for the
		// one we are currently processing before checking if it exists within the git index
		relPath, err := filepath.Rel(g.root, path)
		if err != nil {
			return fmt.Errorf("failed to find root relative path for %v: %w", path, err)
		}

		if !idxCache.hasPath(relPath) {
			log.Debugf("path %s not found in git index, skipping", relPath)
			return nil
		}

		err = filepath.Walk(path, func(path string, info fs.FileInfo, _ error) error {
			// skip directories
			if info.IsDir() {
				return nil
			}

			// determine a path relative to g.root before checking presence in the git index
			relPath, err := filepath.Rel(g.root, path)
			if err != nil {
				return fmt.Errorf("failed to determine a relative path for %s: %w", path, err)
			}

			if !idxCache.hasPath(relPath) {
				log.Debugf("path %v not found in git index, skipping", relPath)
				return nil
			}

			file := File{
				Path:    path,
				RelPath: relPath,
				Info:    info,
			}

			g.stats.Add(stats.Traversed, 1)
			g.filesCh <- &file
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to walk %s: %w", path, err)
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
				err = io.EOF
				break LOOP
			}
			files[idx] = file
			idx++
		}
	}

	return idx, err
}

func (g *GitReader) Close() error {
	return g.eg.Wait()
}

func NewGitReader(
	root string,
	path string,
	statz *stats.Stats,
	batchSize int,
) (*GitReader, error) {
	repo, err := git.PlainOpen(root)
	if err != nil {
		return nil, fmt.Errorf("failed to open git repository: %w", err)
	}

	eg := &errgroup.Group{}

	r := &GitReader{
		root:      root,
		path:      path,
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
