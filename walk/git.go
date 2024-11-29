package walk

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/numtide/treefmt/v2/stats"
	"golang.org/x/sync/errgroup"
)

type GitReader struct {
	root string
	path string

	log   *log.Logger
	stats *stats.Stats

	eg      *errgroup.Group
	scanner *bufio.Scanner
}

func (g *GitReader) Read(ctx context.Context, files []*File) (n int, err error) {
	// ensure we record how many files we traversed
	defer func() {
		g.stats.Add(stats.Traversed, n)
	}()

	if g.scanner == nil {
		// create a pipe to capture the command output
		r, w := io.Pipe()

		// create a command which will execute from the specified sub path within root
		cmd := exec.Command("git", "ls-files")
		cmd.Dir = filepath.Join(g.root, g.path)
		cmd.Stdout = w

		// execute the command in the background
		g.eg.Go(func() error {
			return w.CloseWithError(cmd.Run())
		})

		// create a new scanner for reading the output
		g.scanner = bufio.NewScanner(r)
	}

	nextLine := func() (string, error) {
		line := g.scanner.Text()

		if len(line) == 0 || line[0] != '"' {
			return line, nil
		}

		unquoted, err := strconv.Unquote(line)
		if err != nil {
			return "", fmt.Errorf("failed to unquote line %s: %w", line, err)
		}

		return unquoted, nil
	}

LOOP:

	for n < len(files) {
		select {
		// exit early if the context was cancelled
		case <-ctx.Done():
			err = ctx.Err()
			if err == nil {
				return n, fmt.Errorf("context cancelled: %w", ctx.Err())
			}

			return n, nil

		default:
			// read the next file
			if g.scanner.Scan() {
				entry, err := nextLine()
				if err != nil {
					return n, err
				}

				path := filepath.Join(g.root, g.path, entry)

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
					RelPath: filepath.Join(g.path, entry),
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
	err := g.eg.Wait()
	if err != nil {
		return fmt.Errorf("failed to wait for git command to complete: %w", err)
	}

	return nil
}

func NewGitReader(
	root string,
	path string,
	statz *stats.Stats,
) (*GitReader, error) {
	// check if the root is a git repository
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = root

	if out, err := cmd.Output(); err != nil {
		return nil, fmt.Errorf("failed to check if %s is a git repository: %w", root, err)
	} else if strings.Trim(string(out), "\n") != "true" {
		return nil, fmt.Errorf("%s is not a git repository", root)
	}

	return &GitReader{
		root:  root,
		path:  path,
		stats: statz,
		eg:    &errgroup.Group{},
		log:   log.WithPrefix("walk | git"),
	}, nil
}
