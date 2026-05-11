package walk

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/charmbracelet/log"
	"github.com/numtide/treefmt/v2/git"
	"github.com/numtide/treefmt/v2/stats"
	"golang.org/x/sync/errgroup"
)

type gitEntry struct {
	relative string
	gitlink  bool // mode 160000, i.e. a submodule
}

type GitReader struct {
	root string
	path string

	log   *log.Logger
	stats *stats.Stats

	eg      *errgroup.Group
	filesCh chan *File
	cancel  context.CancelFunc
}

func (g *GitReader) Read(ctx context.Context, files []*File) (n int, err error) {
	defer func() {
		g.stats.Add(stats.Traversed, n)
	}()

LOOP:
	for n < len(files) {
		select {
		case <-ctx.Done():
			return n, ctx.Err() //nolint:wrapcheck

		case file, ok := <-g.filesCh:
			if !ok {
				err = io.EOF

				break LOOP
			}

			files[n] = file
			n++
		}
	}

	return n, err
}

func (g *GitReader) Close() error {
	// Unblock any producer goroutines (and kill the git children) in case the
	// caller stopped draining Read() before EOF.
	g.cancel()

	err := g.eg.Wait()
	if err != nil {
		return fmt.Errorf("failed to wait for git command to complete: %w", err)
	}

	return nil
}

func (g *GitReader) stat(ctx context.Context, entry gitEntry) {
	if entry.gitlink {
		// submodules are separate projects with their own formatting rules
		return
	}

	path := filepath.Join(g.root, entry.relative)

	g.log.Debugf("processing file: %s", path)

	info, err := os.Lstat(path)

	switch {
	case os.IsNotExist(err):
		g.log.Warnf(
			"Path %s is in the worktree but appears to have been removed from the filesystem", path,
		)

		return
	case err != nil:
		g.log.Errorf("failed to stat %s: %v", path, err)

		return
	case info.Mode()&os.ModeSymlink == os.ModeSymlink:
		// symlinks point at files we list anyway
		return
	}

	select {
	case g.filesCh <- &File{Path: path, RelPath: entry.relative, Info: info}:
	case <-ctx.Done():
	}
}

func lsFiles(ctx context.Context, dir string, staged bool, prefix string, out chan<- gitEntry, args ...string) error {
	//nolint:gosec // args are fixed flag sets assembled in NewGitReader, not user input.
	cmd := exec.CommandContext(ctx, "git", append([]string{"ls-files"}, args...)...)
	cmd.Dir = dir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		if ctx.Err() != nil {
			return nil //nolint:nilerr // reader was closed; cancellation is not a failure
		}

		return fmt.Errorf("failed to start git ls-files: %w", err)
	}

	scanErr := scanLsFiles(ctx, stdout, staged, prefix, out)

	// Always reap the child. If scanning aborted early git may be blocked on a
	// full pipe, so kill it first to guarantee Wait returns.
	if scanErr != nil {
		_ = cmd.Process.Kill()
	}

	waitErr := cmd.Wait()

	if ctx.Err() != nil {
		return nil //nolint:nilerr // reader was closed; the kill signal is not a failure
	}

	if scanErr != nil {
		return scanErr
	}

	if waitErr != nil {
		return fmt.Errorf("git ls-files failed: %w", waitErr)
	}

	return nil
}

func scanLsFiles(ctx context.Context, r io.Reader, staged bool, prefix string, out chan<- gitEntry) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()

		var gitlink bool

		path := line
		if staged {
			// <mode> <object> <stage>\t<file>
			if mode, file, ok := strings.Cut(line, "\t"); ok {
				gitlink = strings.HasPrefix(mode, "160000")
				path = file
			}
		}

		if path == "" {
			continue
		}

		if path[0] == '"' {
			unquoted, err := strconv.Unquote(path)
			if err != nil {
				return fmt.Errorf("failed to unquote file %s: %w", path, err)
			}

			path = unquoted
		}

		select {
		case out <- gitEntry{relative: filepath.Join(prefix, path), gitlink: gitlink}:
		case <-ctx.Done():
			return nil
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read git ls-files output: %w", err)
	}

	return nil
}

func NewGitReader(
	root string,
	path string,
	statz *stats.Stats,
) (*GitReader, error) {
	isGit, err := git.IsInsideWorktree(root)
	if err != nil {
		return nil, fmt.Errorf("failed to check if %s is a git repository: %w", root, err)
	}

	if !isGit {
		return nil, fmt.Errorf("%s is not a git repository", root)
	}

	dir := filepath.Join(root, path)

	ctx, cancel := context.WithCancel(context.Background())

	g := &GitReader{
		root:    root,
		path:    path,
		stats:   statz,
		log:     log.WithPrefix("walk | git"),
		eg:      &errgroup.Group{},
		filesCh: make(chan *File, BatchSize*runtime.NumCPU()),
		cancel:  cancel,
	}

	entries := make(chan gitEntry, BatchSize)

	// `--cached` and `--others` are queried separately because git buffers all
	// output until the untracked scan finishes when both are combined; the
	// index-only query streams immediately so formatters start without waiting.
	var producers sync.WaitGroup

	producers.Add(2)

	g.eg.Go(func() error {
		defer producers.Done()

		return lsFiles(ctx, dir, true, path, entries, "--cached", "--stage")
	})

	g.eg.Go(func() error {
		defer producers.Done()

		return lsFiles(ctx, dir, false, path, entries, "--others", "--exclude-standard")
	})

	g.eg.Go(func() error {
		producers.Wait()
		close(entries)

		return nil
	})

	var workers sync.WaitGroup

	statWorkers := runtime.GOMAXPROCS(0)

	workers.Add(statWorkers)

	for range statWorkers {
		g.eg.Go(func() error {
			defer workers.Done()

			for e := range entries {
				g.stat(ctx, e)
			}

			return nil
		})
	}

	g.eg.Go(func() error {
		workers.Wait()
		close(g.filesCh)

		return nil
	})

	return g, nil
}
