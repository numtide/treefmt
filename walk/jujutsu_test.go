package walk_test

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"testing"
	"time"

	"github.com/numtide/treefmt/v2/stats"
	"github.com/numtide/treefmt/v2/test"
	"github.com/numtide/treefmt/v2/walk"
	"github.com/stretchr/testify/require"
)

func TestJujutsuReader(t *testing.T) {
	as := require.New(t)

	test.SetenvXdgConfigDir(t)
	tempDir := test.TempExamples(t)

	// init a jujutsu repo
	cmd := exec.Command("jj", "git", "init")
	cmd.Dir = tempDir
	as.NoError(cmd.Run(), "failed to init jujutsu repository")

	// read empty worktree
	statz := stats.New()
	reader, err := walk.NewJujutsuReader(tempDir, "", &statz)
	as.NoError(err)

	files := make([]*walk.File, 33) // The number of files in `test/examples` used for testing
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	n, err := reader.Read(ctx, files)

	// Jujutsu depends on updating the index with a `jj` command. So, until we do
	// that, the walker should return nothing, since the walker is executed with
	// `--ignore-working-copy` which does not update the index.
	cancel()
	as.Equal(0, n)
	as.ErrorIs(err, io.EOF)

	// update jujutsu's index
	cmd = exec.Command("jj")
	cmd.Dir = tempDir
	as.NoError(cmd.Run(), "failed to update the index")

	statz = stats.New()
	reader, err = walk.NewJujutsuReader(tempDir, "", &statz)
	as.NoError(err)

	count := 0

	for {
		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)

		files := make([]*walk.File, 8)
		n, err := reader.Read(ctx, files)

		count += n

		cancel()

		if errors.Is(err, io.EOF) {
			break
		}
	}

	as.Equal(33, count)
	as.Equal(33, statz.Value(stats.Traversed))
	as.Equal(0, statz.Value(stats.Matched))
	as.Equal(0, statz.Value(stats.Formatted))
	as.Equal(0, statz.Value(stats.Changed))
}
