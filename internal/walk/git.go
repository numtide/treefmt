package walk

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/sync/errgroup"
)

type git struct {
	root string
}

func (g *git) Root() string {
	return g.root
}

func (g *git) Walk(ctx context.Context, fn filepath.WalkFunc) error {
	r, w := io.Pipe()

	cmd := exec.Command("git", "-C", g.root, "ls-files")
	cmd.Stdout = w
	cmd.Stderr = w

	eg := errgroup.Group{}

	eg.Go(func() error {
		scanner := bufio.NewScanner(r)

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				line := scanner.Text()
				path := filepath.Join(g.root, line)

				// stat the file
				info, err := os.Lstat(path)
				if err = fn(path, info, err); err != nil {
					return err
				}
			}
		}

		return nil
	})

	if err := w.CloseWithError(cmd.Run()); err != nil {
		return err
	}

	return eg.Wait()
}

func NewGit(root string) (Walker, error) {
	// check if we're dealing with a git repository
	cmd := exec.Command("git", "-C", root, "rev-parse", "--is-inside-work-tree")
	_, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%w: git repo check failed", err)
	}
	return &git{root}, nil
}
