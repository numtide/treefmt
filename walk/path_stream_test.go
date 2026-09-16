package walk_test

import (
	"context"
	"fmt"
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

func readPathStream(
	t *testing.T,
	reader *walk.PathStreamReader,
	statz *stats.Stats,
	expected int,
) []*walk.File {
	t.Helper()
	as := require.New(t)

	files := make([]*walk.File, 8)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	n, err := reader.Read(ctx, files)

	cancel()

	as.Equal(expected, n)
	as.ErrorIs(err, io.EOF)
	as.Equal(expected, statz.Value(stats.Traversed))

	return files[:n]
}

func TestPathStreamReader(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)

	statz := stats.New()
	reader, err := walk.NewPathStreamReader(tempDir, &statz, walk.PathStreamConfig{
		Name:    "test walker",
		Command: "bash",
		Options: []string{
			"-c",
			"printf '%s\n' go/main.go missing.txt haskell/Foo.hs",
		},
	})
	as.NoError(err)

	files := readPathStream(t, reader, &statz, 2)
	as.Equal("go/main.go", files[0].RelPath)
	as.Equal("haskell/Foo.hs", files[1].RelPath)
	as.NoError(reader.Close())
}

func TestPathStreamReaderSubpath(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)

	statz := stats.New()
	reader, err := walk.NewPathStreamReader(tempDir, &statz, walk.PathStreamConfig{
		Name:    "test walker",
		Command: "bash",
		Options: []string{
			"-c",
			"printf '%s\n' go/main.go haskell/Foo.hs",
		},
		PathFilters: []string{"go"},
	})
	as.NoError(err)

	files := readPathStream(t, reader, &statz, 1)
	as.Equal("go/main.go", files[0].RelPath)
	as.NoError(reader.Close())
}

func TestPathStreamReaderPassesPathFilters(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)

	statz := stats.New()
	reader, err := walk.NewPathStreamReader(tempDir, &statz, walk.PathStreamConfig{
		Name:    "test walker",
		Command: "bash",
		Options: []string{
			"-c",
			`for path in "$@"; do
	case "$path" in
	go) printf '%s\n' go/go.mod go/main.go ;;
	*) printf '%s\n' "$path" ;;
	esac
done`,
			"walker",
		},
		PathFilters: []string{"go"},
	})
	as.NoError(err)

	files := readPathStream(t, reader, &statz, 2)
	as.Equal("go/go.mod", files[0].RelPath)
	as.Equal("go/main.go", files[1].RelPath)
	as.NoError(reader.Close())
}

func TestPathStreamReaderDelimiters(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)

	statz := stats.New()
	reader, err := walk.NewPathStreamReader(tempDir, &statz, walk.PathStreamConfig{
		Name:    "test walker",
		Command: "bash",
		Options: []string{
			"-c",
			`printf 'go/main.go\0haskell/Foo.hs\n'`,
		},
	})
	as.NoError(err)

	files := readPathStream(t, reader, &statz, 2)
	as.Equal("go/main.go", files[0].RelPath)
	as.Equal("haskell/Foo.hs", files[1].RelPath)
	as.NoError(reader.Close())
}

func TestPathStreamReaderCloseUnblocks(t *testing.T) {
	as := require.New(t)

	tempDir := t.TempDir()

	for i := range walk.BatchSize {
		path := fmt.Sprintf("f%04d", i)
		as.NoError(os.WriteFile(filepath.Join(tempDir, path), nil, 0o600))
	}

	statz := stats.New()
	reader, err := walk.NewPathStreamReader(tempDir, &statz, walk.PathStreamConfig{
		Name:    "test walker",
		Command: "bash",
		Options: []string{
			"-c",
			`while true; do for path in "$@"; do printf '%s\n' "$path"; done; done`,
			"walker",
		},
		PathFilters: []string{"."},
	})
	as.NoError(err)

	done := make(chan error, 1)

	go func() { done <- reader.Close() }()

	select {
	case err := <-done:
		as.NoError(err)
	case <-time.After(5 * time.Second):
		t.Fatal("Close() did not return; walker command is deadlocked")
	}
}

func TestPathStreamReaderCommandFailure(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)

	statz := stats.New()
	reader, err := walk.NewPathStreamReader(tempDir, &statz, walk.PathStreamConfig{
		Name:    "test walker",
		Command: "bash",
		Options: []string{
			"-c",
			"printf '%s\n' go/main.go; exit 7",
		},
	})
	as.NoError(err)

	files := readPathStream(t, reader, &statz, 1)
	as.Equal("go/main.go", files[0].RelPath)

	err = reader.Close()
	as.Error(err)
	as.ErrorContains(err, "failed to wait for test walker command to complete")
	as.ErrorContains(err, "exit status 7")
}
