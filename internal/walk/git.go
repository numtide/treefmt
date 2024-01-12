package walk

import (
	"context"
	"fmt"
	"github.com/go-git/go-git/v5"
	"os"
	"path/filepath"
)

type gitWalker struct {
	root string
	repo *git.Repository
}

func (g *gitWalker) Root() string {
	return g.root
}

func (g *gitWalker) Walk(ctx context.Context, fn filepath.WalkFunc) error {

	idx, err := g.repo.Storer.Index()
	if err != nil {
		return fmt.Errorf("%w: failed to open index", err)
	}

	for _, entry := range idx.Entries {

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			path := filepath.Join(g.root, entry.Name)

			// stat the file
			info, err := os.Lstat(path)
			if err = fn(path, info, err); err != nil {
				return err
			}
		}

	}

	return nil
}

func NewGit(root string) (Walker, error) {
	repo, err := git.PlainOpen(root)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to open git repo", err)
	}
	return &gitWalker{root, repo}, nil
}
