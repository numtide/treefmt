package walk

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/numtide/treefmt/stats"
	"golang.org/x/sync/errgroup"
)

type GitReader struct {
	root string
	path string
	args []string

	log   *log.Logger
	stats *stats.Stats

	eg      *errgroup.Group
	scanner *bufio.Scanner
}

func (g *GitReader) Read(ctx context.Context, files []*File) (n int, err error) {
	// ensure we record how many files we traversed
	defer func() {
		g.stats.Add(stats.Traversed, int32(n))
	}()

	if g.scanner == nil {
		// create a pipe to capture the command output
		r, w := io.Pipe()

		// create a command which will execute from the specified sub path within root
		cmd := exec.Command("git", g.args...)
		cmd.Dir = filepath.Join(g.root, g.path)
		cmd.Stdout = w

		// execute the command in the background
		g.eg.Go(func() error {
			return w.CloseWithError(cmd.Run())
		})

		// create a new scanner for reading the output
		g.scanner = bufio.NewScanner(r)
	}

LOOP:

	for n < len(files) {
		select {

		// exit early if the context was cancelled
		case <-ctx.Done():
			return n, ctx.Err()

		default:
			// read the next file
			if g.scanner.Scan() {
				path := filepath.Join(g.root, g.path, g.scanner.Text())

				g.log.Debugf("processing file: %s", path)

				info, err := os.Stat(path)
				if os.IsNotExist(err) {
					// the underlying file might have been removed
					g.log.Warnf(
						"Path %s is in the worktree but appears to have been removed from the filesystem", path,
					)
					continue
				} else if err != nil {
					return n, fmt.Errorf("failed to stat %s: %w", path, err)
				}

				files[n] = &File{
					Path:    path,
					RelPath: filepath.Join(g.path, g.scanner.Text()),
					Info:    info,
				}
				n++

			} else {
				// nothing more to read
				err = io.EOF
				break LOOP
			}
		}
	}

	return n, err
}

func (g *GitReader) Close() error {
	return g.eg.Wait()
}

func newGitReader(
	root string,
	path string,
	args []string,
	statz *stats.Stats,
) (*GitReader, error) {
	// check if the root is a git repository
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = root

	if out, err := cmd.Output(); err != nil {
		return nil, fmt.Errorf("failed to check if git repository is inside work tree: %w", err)
	} else if strings.Trim(string(out), "\n") != "true" {
		return nil, fmt.Errorf("git repository is not inside work tree")
	}

	return &GitReader{
		root:  root,
		path:  path,
		args:  args,
		stats: statz,
		eg:    &errgroup.Group{},
		log:   log.WithPrefix("walk[git]"),
	}, nil
}

func NewGitStagedReader(
	root string,
	path string,
	statz *stats.Stats,
) (*GitReader, error) {
	return newGitReader(root, path, []string{"diff", "--name-only", "--cached"}, statz)
}

func NewGitIndexReader(
	root string,
	path string,
	statz *stats.Stats,
) (*GitReader, error) {
	return newGitReader(root, path, []string{"ls-files"}, statz)
}
