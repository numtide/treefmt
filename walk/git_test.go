package walk_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/numtide/treefmt/v2/stats"
	"github.com/numtide/treefmt/v2/test"
	"github.com/numtide/treefmt/v2/walk"
	"github.com/stretchr/testify/require"
)

func TestGitReader(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)

	// init a git repo
	cmd := exec.CommandContext(t.Context(), "git", "init")
	cmd.Dir = tempDir
	as.NoError(cmd.Run(), "failed to init git repository")

	// configure git username and email locally
	cmd = exec.CommandContext(t.Context(), "git", "config", "user.name", "testing")
	cmd.Dir = tempDir
	as.NoError(cmd.Run(), "failed to set git username")

	cmd = exec.CommandContext(t.Context(), "git", "config", "user.email", "testing@example.com")
	cmd.Dir = tempDir
	as.NoError(cmd.Run(), "failed to set git email")

	// read empty worktree
	statz := stats.New()
	reader, err := walk.NewGitReader(tempDir, "", &statz)
	as.NoError(err)

	files := make([]*walk.File, 34)
	ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
	n, err := reader.Read(ctx, files)

	cancel()
	as.Equal(33, n)
	as.ErrorIs(err, io.EOF)

	// add a git submodule
	tempSubmoduleDir := test.TempExamples(t)
	cmd = exec.CommandContext(t.Context(), "git", "init")
	cmd.Dir = tempSubmoduleDir
	as.NoError(cmd.Run(), "failed to init git submodule repository")

	// configure git username and email locally for the submodule
	cmd = exec.CommandContext(t.Context(), "git", "config", "user.name", "testing")
	cmd.Dir = tempSubmoduleDir
	as.NoError(cmd.Run(), "failed to set submodule git username")

	cmd = exec.CommandContext(t.Context(), "git", "config", "user.email", "testing@example.com")
	cmd.Dir = tempSubmoduleDir
	as.NoError(cmd.Run(), "failed to set submodule git email")

	// add everything to the submodule's git index
	cmd = exec.CommandContext(t.Context(), "git", "add", ".")
	cmd.Dir = tempSubmoduleDir
	as.NoError(cmd.Run(), "failed to add everything to the submodule index")

	// commit the submodule
	cmd = exec.CommandContext(t.Context(), "git", "commit", "-m", "submodule")
	cmd.Dir = tempSubmoduleDir
	as.NoError(cmd.Run(), "failed to commit the submodule")

	// add the submodule to the main git repository
	// https://github.blog/open-source/git/git-security-vulnerabilities-announced/#cve-2022-39253
	// use -c to pass protocol.file.allow since submodule clone spawns a subprocess that won't see local config
	cmd = exec.CommandContext(t.Context(), "git", "-c", "protocol.file.allow=always", "submodule", "add", tempSubmoduleDir)
	cmd.Dir = tempDir
	as.NoError(cmd.Run(), "failed to add the submodule to the main repository")

	// add everything to the git index
	cmd = exec.CommandContext(t.Context(), "git", "add", ".")
	cmd.Dir = tempDir
	as.NoError(cmd.Run(), "failed to add everything to the index")

	statz = stats.New()
	reader, err = walk.NewGitReader(tempDir, "", &statz)
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

	as.Equal(34, count)
	as.Equal(34, statz.Value(stats.Traversed))
	as.Equal(0, statz.Value(stats.Matched))
	as.Equal(0, statz.Value(stats.Formatted))
	as.Equal(0, statz.Value(stats.Changed))
}

// TestGitReaderCloseUnblocks verifies that Close() returns even when the
// caller stops draining Read() before EOF.
func TestGitReaderCloseUnblocks(t *testing.T) {
	as := require.New(t)

	tempDir := t.TempDir()

	cmd := exec.CommandContext(t.Context(), "git", "init")
	cmd.Dir = tempDir
	as.NoError(cmd.Run())

	// Enough files to overflow the reader's internal channel so producers
	// would block if Close() did not unblock them.
	n := walk.BatchSize*runtime.GOMAXPROCS(0) + 100
	for i := range n {
		as.NoError(os.WriteFile(filepath.Join(tempDir, fmt.Sprintf("f%05d", i)), nil, 0o600))
	}

	cmd = exec.CommandContext(t.Context(), "git", "add", ".")
	cmd.Dir = tempDir
	as.NoError(cmd.Run())

	statz := stats.New()

	reader, err := walk.NewGitReader(tempDir, "", &statz)
	as.NoError(err)

	done := make(chan error, 1)

	go func() { done <- reader.Close() }()

	select {
	case err := <-done:
		as.NoError(err)
	case <-time.After(5 * time.Second):
		t.Fatal("Close() did not return; producers are deadlocked")
	}
}
