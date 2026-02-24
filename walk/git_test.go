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

func TestGitReader(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)

	// configure git username and email
	cmd := exec.Command("git", "config", "--global", "user.name", "testing")
	cmd.Dir = tempDir
	as.NoError(cmd.Run(), "failed to set git username")
	cmd = exec.Command("git", "config", "--global", "user.email", "testing@example.com")
	cmd.Dir = tempDir
	as.NoError(cmd.Run(), "failed to set git email")
	// https://github.blog/open-source/git/git-security-vulnerabilities-announced/#cve-2022-39253
	// We only use submodules we trust
	cmd = exec.Command("git", "config", "--global", "protocol.file.allow", "always")
	cmd.Dir = tempDir
	as.NoError(cmd.Run(), "failed to allow file protocol")

	// init a git repo
	cmd = exec.Command("git", "init")
	cmd.Dir = tempDir
	as.NoError(cmd.Run(), "failed to init git repository")

	// read empty worktree
	statz := stats.New()
	reader, err := walk.NewGitReader(tempDir, "", &statz)
	as.NoError(err)

	files := make([]*walk.File, 35)
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	n, err := reader.Read(ctx, files)

	cancel()
	as.Equal(34, n)
	as.ErrorIs(err, io.EOF)

	// add a git submodule
	tempSubmoduleDir := test.TempExamples(t)
	cmd = exec.Command("git", "init")
	cmd.Dir = tempSubmoduleDir
	as.NoError(cmd.Run(), "failed to init git submodule repository")

	// add everything to the submodule's git index
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tempSubmoduleDir
	as.NoError(cmd.Run(), "failed to add everything to the submodule index")

	// commit the submodule
	cmd = exec.Command("git", "commit", "-m", "submodule")
	cmd.Dir = tempSubmoduleDir
	as.NoError(cmd.Run(), "failed to commit the submodule")

	// add the submodule to the main git repository
	cmd = exec.Command("git", "submodule", "add", tempSubmoduleDir)
	cmd.Dir = tempDir
	as.NoError(cmd.Run(), "failed to add the submodule to the main repository")

	// add everything to the git index
	cmd = exec.Command("git", "add", ".")
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
