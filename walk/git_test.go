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

	"github.com/numtide/treefmt/stats"
	"github.com/numtide/treefmt/test"
	"github.com/numtide/treefmt/walk"
	"github.com/stretchr/testify/require"
)

func TestGitIndexReader(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)

	// init a git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	as.NoError(cmd.Run(), "failed to init git repository")

	// read empty worktree
	statz := stats.New()
	reader, err := walk.NewGitIndexReader(tempDir, "", &statz)
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

	reader, err = walk.NewGitIndexReader(tempDir, "", &statz)
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
	as.Equal(int32(0), statz.Value(stats.Emitted))
	as.Equal(int32(0), statz.Value(stats.Matched))
	as.Equal(int32(0), statz.Value(stats.Formatted))
}

func TestGitStagedReader(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)

	// init a git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	as.NoError(cmd.Run(), "failed to init git repository")

	// add everything to the git index
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tempDir
	as.NoError(cmd.Run(), "failed to add everything to the index")

	// commit
	cmd = exec.Command("git", "commit", "-m", "initial commit")
	cmd.Dir = tempDir
	as.NoError(cmd.Run(), "failed to commit")

	// should be empty
	statz := stats.New()
	reader, err := walk.NewGitStagedReader(tempDir, "", &statz)
	as.NoError(err)

	files := make([]*walk.File, 8)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	n, err := reader.Read(ctx, files)

	cancel()
	as.Equal(0, n)
	as.ErrorIs(err, io.EOF)

	// stage some changes

	appendToFile := func(path string, content string) {
		f, err := os.OpenFile(filepath.Join(tempDir, path), os.O_APPEND|os.O_WRONLY, 0o644)
		as.NoError(err)
		defer f.Close()
		_, err = f.WriteString(content)
		as.NoError(err)
	}

	appendToFile("treefmt.toml", "hello")
	appendToFile("rust/Cargo.toml", "foo")
	appendToFile("nix/sources.nix", "bar")

	_, err = os.Create(filepath.Join(tempDir, "new.txt"))
	as.NoError(err)

	// add to the index

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tempDir
	as.NoError(cmd.Run(), "failed to add everything to the index")

	// read changes

	statz = stats.New()
	reader, err = walk.NewGitStagedReader(tempDir, "", &statz)
	as.NoError(err)

	ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)

	files = make([]*walk.File, 8)
	n, err = reader.Read(ctx, files)
	cancel()

	as.Equal(4, n)
	as.ErrorIs(err, io.EOF)

	as.Equal("new.txt", files[0].Path)
	as.Equal("nix/sources.nix", files[1].Path)
	as.Equal("rust/Cargo.toml", files[2].Path)
	as.Equal("treefmt.toml", files[3].Path)

	as.Equal(int32(4), statz.Value(stats.Traversed))
	as.Equal(int32(0), statz.Value(stats.Emitted))
	as.Equal(int32(0), statz.Value(stats.Matched))
	as.Equal(int32(0), statz.Value(stats.Formatted))
}
