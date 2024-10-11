package walk_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/numtide/treefmt/stats"

	"github.com/numtide/treefmt/walk"

	"github.com/numtide/treefmt/test"
	"github.com/stretchr/testify/require"
)

var examplesPaths = []string{
	"elm/elm.json",
	"elm/src/Main.elm",
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

func TestFilesystemReader(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)
	statz := stats.New()

	r := walk.NewFilesystemReader(tempDir, nil, &statz, 1024)

	count := 0

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)

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

	as.Equal(32, count)
	as.Equal(int32(32), statz.Value(stats.Traversed))
	as.Equal(int32(0), statz.Value(stats.Emitted))
	as.Equal(int32(0), statz.Value(stats.Matched))
	as.Equal(int32(0), statz.Value(stats.Formatted))
}
