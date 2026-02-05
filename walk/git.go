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
	"github.com/numtide/treefmt/v2/git"
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
		cmd := exec.Command("git", "ls-files", "--cached", "--others", "--exclude-standard", "--stage")
		cmd.Dir = filepath.Join(g.root, g.path)
		cmd.Stdout = w

		// execute the command in the background
		g.eg.Go(func() error {
			return w.CloseWithError(cmd.Run())
		})

		// create a new scanner for reading the output
		g.scanner = bufio.NewScanner(r)
	}

	nextFile := func() (string, error) {
		for line := g.scanner.Text(); len(line) > 0; line = g.scanner.Text() {
			lineSplit := strings.Split(line, "\t")

			var stage, file string
			// Untracked files just show as `<filename>`, while tracked files show as `<mode> <object> <stage><TAB><file>`
			if len(lineSplit) == 1 {
				stage, file = "", lineSplit[0]
			} else {
				stage, file = lineSplit[0], lineSplit[1]
			}

			// 160000 is the mode for submodules, skip them because they are separate projects that may have their own
			// formatting rules
			if strings.HasPrefix(stage, "160000") {
				g.scanner.Scan()

				continue
			}

			if file[0] != '"' {
				return file, nil
			}

			unquoted, err := strconv.Unquote(file)
			if err != nil {
				return "", fmt.Errorf("failed to unquote file %s: %w", file, err)
			}

			return unquoted, nil
		}

		return "", io.EOF
	}

LOOP:

	for n < len(files) {
		select {
		// exit early if the context was cancelled
		case <-ctx.Done():
			return n, ctx.Err() //nolint:wrapcheck

		default:
			// read the next file
			if g.scanner.Scan() {
				entry, err := nextFile()
				if err != nil {
					return n, err
				}

				path := filepath.Join(g.root, g.path, entry)

				g.log.Debugf("processing file: %s", path)

				info, err := os.Lstat(path)

				switch {
				case os.IsNotExist(err):
					// the underlying file might have been removed
					g.log.Warnf(
						"Path %s is in the worktree but appears to have been removed from the filesystem", path,
					)

					continue
				case err != nil:
					return n, fmt.Errorf("failed to stat %s: %w", path, err)
				case info.Mode()&os.ModeSymlink == os.ModeSymlink:
					// we skip reporting symlinks stored in Git, they should
					// point to local files which we would list anyway.
					continue
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
	isGit, err := git.IsInsideWorktree(root)
	if err != nil {
		return nil, fmt.Errorf("failed to check if %s is a git repository: %w", root, err)
	}

	if !isGit {
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
