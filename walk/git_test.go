package walk_test

import (
	"context"
	"errors"
	"io"
	"path"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/numtide/treefmt/stats"
	"github.com/numtide/treefmt/test"
	"github.com/numtide/treefmt/walk"
	"github.com/stretchr/testify/require"
)

func TestGitWorktreeReader(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)

	// init a git repo
	repo, err := git.Init(
		filesystem.NewStorage(
			osfs.New(path.Join(tempDir, ".git")),
			cache.NewObjectLRUDefault(),
		),
		osfs.New(tempDir),
	)
	as.NoError(err, "failed to init git repository")

	// get worktree and add everything to it
	wt, err := repo.Worktree()
	as.NoError(err, "failed to get git worktree")
	as.NoError(wt.AddGlob("."))

	statz := stats.New()

	reader, err := walk.NewGitWorktreeReader(tempDir, "", &statz)
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
