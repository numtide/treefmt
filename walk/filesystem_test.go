package walk_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/numtide/treefmt/v2/stats"
	"github.com/numtide/treefmt/v2/test"
	"github.com/numtide/treefmt/v2/walk"
	"github.com/stretchr/testify/require"
)

//nolint:gochecknoglobals
var examplesPaths = []string{
	".gitignore",
	"emoji 🕰️/README.md",
	"go/go.mod",
	"go/main.go",
	"haskell/CHANGELOG.md",
	"haskell/Foo.hs",
	"haskell/Main.hs",
	"haskell/Nested/Foo.hs",
	"haskell/Setup.hs",
	"haskell/haskell.cabal",
	"haskell/treefmt.toml",
	"haskell-frontend/CHANGELOG.md",
	"haskell-frontend/Main.hs",
	"haskell-frontend/Setup.hs",
	"haskell-frontend/haskell-frontend.cabal",
	"html/index.html",
	"html/scripts/.gitkeep",
	"javascript/source/hello.js",
	"just/justfile",
	"nix/sources.nix",
	"nixpkgs.toml",
	"python/main.py",
	"python/requirements.txt",
	"python/virtualenv_proxy.py",
	"ruby/bundler.rb",
	"rust/Cargo.toml",
	"rust/src/main.rs",
	"shell/foo.sh",
	"terraform/main.tf",
	"terraform/two.tf",
	"touch.toml",
	"treefmt.toml",
	"yaml/test.yaml",
}

func TestFilesystemReaderCancellation(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)
	statz := stats.New()

	r := walk.NewFilesystemReader(tempDir, "", &statz, 1024)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	files := make([]*walk.File, 8)
	_, err := r.Read(ctx, files)
	as.ErrorIs(err, context.Canceled)
}

func TestFilesystemReader(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)
	statz := stats.New()

	r := walk.NewFilesystemReader(tempDir, "", &statz, 1024)

	count := 0

	for {
		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)

		files := make([]*walk.File, 8)
		n, err := r.Read(ctx, files)

		for i := count; i < count+n; i++ {
			as.Equal(examplesPaths[i], files[i-count].RelPath)
		}

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

// TestFilesystemReaderCloseUnblocks verifies that Close() returns even when
// the caller stops draining Read() before EOF.
func TestFilesystemReaderCloseUnblocks(t *testing.T) {
	as := require.New(t)

	tempDir := t.TempDir()

	// Enough files to overflow the reader's internal channel so process()
	// would block on send if Close() did not unblock it.
	n := walk.BatchSize*runtime.GOMAXPROCS(0) + 100
	for i := range n {
		as.NoError(os.WriteFile(filepath.Join(tempDir, fmt.Sprintf("f%05d", i)), nil, 0o600))
	}

	statz := stats.New()
	reader := walk.NewFilesystemReader(tempDir, "", &statz, walk.BatchSize)

	done := make(chan error, 1)

	go func() { done <- reader.Close() }()

	select {
	case err := <-done:
		as.NoError(err)
	case <-time.After(5 * time.Second):
		t.Fatal("Close() did not return; process() is deadlocked")
	}
}
