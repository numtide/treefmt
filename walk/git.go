package walk

import (
	"fmt"

	"github.com/numtide/treefmt/v2/git"
	"github.com/numtide/treefmt/v2/stats"
)

type GitReader = PathStreamReader

func NewGitReader(
	root string,
	pathFilters []string,
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

	return NewPathStreamReader(root, statz, PathStreamConfig{
		Name:        "git",
		Command:     "git",
		Options:     []string{"ls-files", "-z", "--cached", "--others", "--exclude-standard", "--full-name", "--"},
		PathFilters: pathFilters,
	})
}
