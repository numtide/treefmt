package walker

import (
	"context"
	"os"
	"testing"

	"git.numtide.com/numtide/treefmt/test"
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

func TestFilesystemWalker_Walk(t *testing.T) {
	tempDir := test.TempExamples(t)

	paths := make(chan string, 1)
	go func() {
		paths <- tempDir
		close(paths)
	}()

	as := require.New(t)

	walker, err := NewFilesystem(tempDir, paths)
	as.NoError(err)

	idx := 0
	err = walker.Walk(context.Background(), func(file *File, err error) error {
		as.Equal(examplesPaths[idx], file.RelPath)
		idx += 1
		return nil
	})
	as.NoError(err)

	// capture current cwd, so we can replace it after the test is finished
	cwd, err := os.Getwd()
	as.NoError(err)
	t.Cleanup(func() {
		// return to the previous working directory
		as.NoError(os.Chdir(cwd))
	})
}
