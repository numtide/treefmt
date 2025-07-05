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

	"github.com/charmbracelet/log"
	"github.com/numtide/treefmt/v2/jujutsu"
	"github.com/numtide/treefmt/v2/stats"
	"golang.org/x/sync/errgroup"
)

type JujutsuReader struct {
	root string
	path string

	log   *log.Logger
	stats *stats.Stats

	eg      *errgroup.Group
	scanner *bufio.Scanner
}

func (j *JujutsuReader) Read(ctx context.Context, files []*File) (n int, err error) {
	// ensure we record how many files we traversed
	defer func() {
		j.stats.Add(stats.Traversed, n)
	}()

	if j.scanner == nil {
		// create a pipe to capture the command output
		r, w := io.Pipe()

		// create a command which will execute from root
		// --ignore-working-copy: Don't snapshot the working copy, and don't update it. This prevents that the user has to
		// enter a password for singning the commit. New files also won't be added to the index and not displayed in the
		// output.
		// Add the subpath as a fileset displaying only files matching this prefix. If
		// the subpath is empty ignore it since it interferese with command
		args := []string{"file", "list", "--ignore-working-copy"}
		if j.path != "" {
			args = append(args, j.path)
		}

		cmd := exec.Command("jj", args...)
		cmd.Dir = j.root
		cmd.Stdout = w

		// execute the command in the background
		j.eg.Go(func() error {
			return w.CloseWithError(cmd.Run())
		})

		// create a new scanner for reading the output
		j.scanner = bufio.NewScanner(r)
	}

	nextLine := func() (string, error) {
		line := j.scanner.Text()

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
			if j.scanner.Scan() {
				entry, err := nextLine()
				if err != nil {
					return n, err
				}

				path := filepath.Join(j.root, entry)

				j.log.Debugf("processing file: %s", path)

				info, err := os.Lstat(path)

				switch {
				case os.IsNotExist(err):
					// the underlying file might have been removed
					j.log.Warnf(
						"Path %s is in the worktree but appears to have been removed from the filesystem", path,
					)

					continue
				case err != nil:
					return n, fmt.Errorf("failed to stat %s: %w", path, err)
				case info.Mode()&os.ModeSymlink == os.ModeSymlink:
					// we skip reporting symlinks stored in Jujutsu, they should
					// point to local files which we would list anyway.
					continue
				}

				files[n] = &File{
					Path:    path,
					RelPath: entry,
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

func (j *JujutsuReader) Close() error {
	err := j.eg.Wait()
	if err != nil {
		return fmt.Errorf("failed to wait for jujutsu command to complete: %w", err)
	}

	return nil
}

func NewJujutsuReader(
	root string,
	path string,
	statz *stats.Stats,
) (*JujutsuReader, error) {
	// check if the root is a jujutsu repository
	isJujutsu, err := jujutsu.IsInsideWorktree(root)
	if err != nil {
		return nil, fmt.Errorf("failed to check if %s is a jujutsu repository: %w", root, err)
	}

	if !isJujutsu {
		return nil, fmt.Errorf("%s is not a jujutsu repository", root)
	}

	return &JujutsuReader{
		root:  root,
		path:  path,
		stats: statz,
		eg:    &errgroup.Group{},
		log:   log.WithPrefix("walk | jujutsu"),
	}, nil
}
