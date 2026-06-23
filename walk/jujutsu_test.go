package walk_test

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
	cmd := exec.CommandContext(t.Context(), "jj", "git", "init")
	cmd.Dir = tempDir
	as.NoError(cmd.Run(), "failed to init jujutsu repository")

	// read empty worktree
	statz := stats.New()
	reader, err := walk.NewJujutsuReader(tempDir, nil, &statz)
	as.NoError(err)

	files := make([]*walk.File, 33) // The number of files in `test/examples` used for testing
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	n, err := reader.Read(ctx, files)

	// Jujutsu depends on updating the index with a `jj` command. So, until we do
	// that, the walker should return nothing, since the walker is executed with
	// `--ignore-working-copy` which does not update the index.
	cancel()
	as.Equal(0, n)
	as.ErrorIs(err, io.EOF)

	// update jujutsu's index
	cmd = exec.CommandContext(t.Context(), "jj")
	cmd.Dir = tempDir
	as.NoError(cmd.Run(), "failed to update the index")

	statz = stats.New()
	reader, err = walk.NewJujutsuReader(tempDir, nil, &statz)
	as.NoError(err)

	count := 0

	for {
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)

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

func TestJujutsuReaderUsesTreeRoot(t *testing.T) {
	as := require.New(t)

	test.SetenvXdgConfigDir(t)
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "my-project")
	otherDir := filepath.Join(tempDir, "other-project")

	as.NoError(os.MkdirAll(projectDir, 0o755))
	as.NoError(os.MkdirAll(otherDir, 0o755))
	as.NoError(os.WriteFile(filepath.Join(projectDir, "default.nix"), []byte("{}\n"), 0o600))
	as.NoError(os.WriteFile(filepath.Join(otherDir, "shell.nix"), []byte("{}\n"), 0o600))

	cmd := exec.CommandContext(t.Context(), "jj", "git", "init", "--no-colocate")
	cmd.Dir = tempDir
	as.NoError(cmd.Run(), "failed to init jujutsu repository")

	cmd = exec.CommandContext(t.Context(), "jj", "file", "track", "my-project/default.nix", "other-project/shell.nix")
	cmd.Dir = tempDir
	as.NoError(cmd.Run(), "failed to track files")

	statz := stats.New()
	reader, err := walk.NewJujutsuReader(projectDir, nil, &statz)
	as.NoError(err)

	defer func() {
		as.NoError(reader.Close())
	}()

	files := make([]*walk.File, 8)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	n, err := reader.Read(ctx, files)

	cancel()

	as.Equal(1, n)
	as.ErrorIs(err, io.EOF)
	as.Equal("default.nix", files[0].RelPath)
	as.Equal(1, statz.Value(stats.Traversed))
}
