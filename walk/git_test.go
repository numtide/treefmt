package walk_test

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"testing"
	"time"

	"github.com/numtide/treefmt/stats"
	"github.com/numtide/treefmt/test"
	"github.com/numtide/treefmt/walk"
	"github.com/stretchr/testify/require"
)

func TestGitReader(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)

	// init a git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	as.NoError(cmd.Run(), "failed to init git repository")

	// read empty worktree
	statz := stats.New()
	reader, err := walk.NewGitReader(tempDir, "", &statz)
	as.NoError(err)

	files := make([]*walk.File, 8)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	n, err := reader.Read(ctx, files)

	cancel()
	as.Equal(0, n)
	as.ErrorIs(err, io.EOF)

	// add everything to the git index
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tempDir
	as.NoError(cmd.Run(), "failed to add everything to the index")

	reader, err = walk.NewGitReader(tempDir, "", &statz)
	as.NoError(err)

	count := 0

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)

		files := make([]*walk.File, 8)
		n, err := reader.Read(ctx, files)

		count += n

		cancel()

		if errors.Is(err, io.EOF) {
			break
		}
	}

	as.Equal(32, count)
	as.Equal(int32(32), statz.Value(stats.Traversed))
	as.Equal(int32(0), statz.Value(stats.Matched))
	as.Equal(int32(0), statz.Value(stats.Formatted))
	as.Equal(int32(0), statz.Value(stats.Changed))
}
