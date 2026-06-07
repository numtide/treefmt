package walk

import (
	"fmt"

	"github.com/numtide/treefmt/v2/jujutsu"
	"github.com/numtide/treefmt/v2/stats"
)

type JujutsuReader = PathStreamReader

func NewJujutsuReader(
	root string,
	pathFilters []string,
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

	// --ignore-working-copy: Don't snapshot the working copy, and don't update it. This prevents that the user has to
	// enter a password for signing the commit. New files also won't be added to the index and not displayed in the
	// output.
	return NewPathStreamReader(root, statz, PathStreamConfig{
		Name:        "jujutsu",
		Command:     "jj",
		Options:     []string{"file", "list", "--ignore-working-copy", "--"},
		PathFilters: pathFilters,
	})
}
