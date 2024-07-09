package walk

import (
	"context"
	"os"
	"testing"

	"git.numtide.com/numtide/treefmt/test"
	"github.com/stretchr/testify/require"
)

var examplesPaths = []string{
	".",
	"elm",
	"elm/elm.json",
	"elm/src",
	"elm/src/Main.elm",
	"go",
	"go/go.mod",
	"go/main.go",
	"haskell",
	"haskell/CHANGELOG.md",
	"haskell/Foo.hs",
	"haskell/Main.hs",
	"haskell/Nested",
	"haskell/Nested/Foo.hs",
	"haskell/Setup.hs",
	"haskell/haskell.cabal",
	"haskell/treefmt.toml",
	"haskell-frontend",
	"haskell-frontend/CHANGELOG.md",
	"haskell-frontend/Main.hs",
	"haskell-frontend/Setup.hs",
	"haskell-frontend/haskell-frontend.cabal",
	"html",
	"html/index.html",
	"html/scripts",
	"html/scripts/.gitkeep",
	"javascript",
	"javascript/source",
	"javascript/source/hello.js",
	"nix",
	"nix/sources.nix",
	"nixpkgs.toml",
	"python",
	"python/main.py",
	"python/requirements.txt",
	"python/virtualenv_proxy.py",
	"ruby",
	"ruby/bundler.rb",
	"rust",
	"rust/Cargo.toml",
	"rust/src",
	"rust/src/main.rs",
	"shell",
	"shell/foo.sh",
	"terraform",
	"terraform/main.tf",
	"terraform/two.tf",
	"touch.toml",
	"treefmt.toml",
	"yaml",
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
