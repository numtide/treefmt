package walk_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/numtide/treefmt/v2/stats"
	"github.com/numtide/treefmt/v2/test"
	"github.com/numtide/treefmt/v2/walk"
	"github.com/stretchr/testify/require"
)

func TestCustomReader(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)

	walkerPath := filepath.Join(tempDir, "walker")
	as.NoError(os.WriteFile(walkerPath, []byte(`#!/usr/bin/env sh
for path in "$@"; do
  printf '%s\n' "$path"
done
`), 0o700))

	as.NoError(os.Symlink(filepath.Join(tempDir, "go", "main.go"), filepath.Join(tempDir, "link-to-main")))

	statz := stats.New()
	reader, err := walk.NewCustomReader(tempDir, "", &statz, walk.CustomConfig{
		Name:    "myWalker",
		Command: "./walker",
		Options: []string{
			"go/main.go",
			"go/go.mod",
			"go",
			"missing.txt",
			"link-to-main",
		},
	})
	as.NoError(err)

	files := make([]*walk.File, 8)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	n, err := reader.Read(ctx, files)
	cancel()

	as.Equal(2, n)
	as.ErrorIs(err, io.EOF)
	as.Equal("go/main.go", files[0].RelPath)
	as.Equal("go/go.mod", files[1].RelPath)
	as.Equal(2, statz.Value(stats.Traversed))
	as.NoError(reader.Close())
}

func TestCustomReaderSubpath(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)

	walkerPath := filepath.Join(tempDir, "walker")
	as.NoError(os.WriteFile(walkerPath, []byte(`#!/usr/bin/env sh
printf '%s\n' go/main.go haskell/Foo.hs go/go.mod
`), 0o700))

	statz := stats.New()
	reader, err := walk.NewCustomReader(tempDir, "go", &statz, walk.CustomConfig{
		Name:    "myWalker",
		Command: "./walker",
	})
	as.NoError(err)

	files := make([]*walk.File, 8)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	n, err := reader.Read(ctx, files)
	cancel()

	as.Equal(2, n)
	as.ErrorIs(err, io.EOF)
	as.Equal("go/main.go", files[0].RelPath)
	as.Equal("go/go.mod", files[1].RelPath)
	as.Equal(2, statz.Value(stats.Traversed))
	as.NoError(reader.Close())
}

func TestCustomReaderCommandFailure(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)

	walkerPath := filepath.Join(tempDir, "walker")
	as.NoError(os.WriteFile(walkerPath, []byte(`#!/usr/bin/env sh
printf '%s\n' go/main.go
exit 7
`), 0o700))

	statz := stats.New()
	reader, err := walk.NewCustomReader(tempDir, "", &statz, walk.CustomConfig{
		Name:    "myWalker",
		Command: "./walker",
	})
	as.NoError(err)

	files := make([]*walk.File, 8)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	n, err := reader.Read(ctx, files)
	cancel()

	as.Equal(1, n)
	as.ErrorIs(err, io.EOF)
	as.Equal("go/main.go", files[0].RelPath)

	err = reader.Close()
	as.Error(err)
	as.ErrorContains(err, "failed to wait for custom walker myWalker command to complete")
	as.ErrorContains(err, "exit status 7")
}
