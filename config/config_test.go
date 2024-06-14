package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadConfigFile(t *testing.T) {
	as := require.New(t)

	cfg, err := ReadFile("../test/examples/treefmt.toml", nil)
	as.NoError(err, "failed to read config file")

	as.NotNil(cfg)

	as.Equal([]string{"*.toml"}, cfg.Global.Excludes)

	// python
	python, ok := cfg.Formatters["python"]
	as.True(ok, "python formatter not found")
	as.Equal("black", python.Command)
	as.Nil(python.Options)
	as.Equal([]string{"*.py"}, python.Includes)
	as.Nil(python.Excludes)

	// elm
	elm, ok := cfg.Formatters["elm"]
	as.True(ok, "elm formatter not found")
	as.Equal("elm-format", elm.Command)
	as.Equal([]string{"--yes"}, elm.Options)
	as.Equal([]string{"*.elm"}, elm.Includes)
	as.Nil(elm.Excludes)

	// go
	golang, ok := cfg.Formatters["go"]
	as.True(ok, "go formatter not found")
	as.Equal("gofmt", golang.Command)
	as.Equal([]string{"-w"}, golang.Options)
	as.Equal([]string{"*.go"}, golang.Includes)
	as.Nil(golang.Excludes)

	// haskell
	haskell, ok := cfg.Formatters["haskell"]
	as.True(ok, "haskell formatter not found")
	as.Equal("ormolu", haskell.Command)
	as.Equal([]string{
		"--ghc-opt", "-XBangPatterns",
		"--ghc-opt", "-XPatternSynonyms",
		"--ghc-opt", "-XTypeApplications",
		"--mode", "inplace",
		"--check-idempotence",
	}, haskell.Options)
	as.Equal([]string{"*.hs"}, haskell.Includes)
	as.Equal([]string{"examples/haskell/"}, haskell.Excludes)

	// alejandra
	alejandra, ok := cfg.Formatters["alejandra"]
	as.True(ok, "alejandra formatter not found")
	as.Equal("alejandra", alejandra.Command)
	as.Nil(alejandra.Options)
	as.Equal([]string{"*.nix"}, alejandra.Includes)
	as.Equal([]string{"examples/nix/sources.nix"}, alejandra.Excludes)
	as.Equal(1, alejandra.Priority)

	// deadnix
	deadnix, ok := cfg.Formatters["deadnix"]
	as.True(ok, "deadnix formatter not found")
	as.Equal("deadnix", deadnix.Command)
	as.Equal([]string{"-e"}, deadnix.Options)
	as.Equal([]string{"*.nix"}, deadnix.Includes)
	as.Nil(deadnix.Excludes)
	as.Equal(2, deadnix.Priority)

	// ruby
	ruby, ok := cfg.Formatters["ruby"]
	as.True(ok, "ruby formatter not found")
	as.Equal("rufo", ruby.Command)
	as.Equal([]string{"-x"}, ruby.Options)
	as.Equal([]string{"*.rb"}, ruby.Includes)
	as.Nil(ruby.Excludes)

	// prettier
	prettier, ok := cfg.Formatters["prettier"]
	as.True(ok, "prettier formatter not found")
	as.Equal("prettier", prettier.Command)
	as.Equal([]string{"--write", "--tab-width", "4"}, prettier.Options)
	as.Equal([]string{
		"*.css",
		"*.html",
		"*.js",
		"*.json",
		"*.jsx",
		"*.md",
		"*.mdx",
		"*.scss",
		"*.ts",
		"*.yaml",
	}, prettier.Includes)
	as.Equal([]string{"CHANGELOG.md"}, prettier.Excludes)

	// rust
	rust, ok := cfg.Formatters["rust"]
	as.True(ok, "rust formatter not found")
	as.Equal("rustfmt", rust.Command)
	as.Equal([]string{"--edition", "2018"}, rust.Options)
	as.Equal([]string{"*.rs"}, rust.Includes)
	as.Nil(rust.Excludes)

	// shell
	shell, ok := cfg.Formatters["shell"]
	as.True(ok, "shell formatter not found")
	as.Equal("/bin/sh", shell.Command)
	as.Equal([]string{
		"-euc",
		`# First lint all the scripts
shellcheck "$@"

# Then format them
shfmt -i 2 -s -w "$@"
    `,
		"--",
	}, shell.Options)
	as.Equal([]string{"*.sh"}, shell.Includes)
	as.Nil(shell.Excludes)

	// terraform
	terraform, ok := cfg.Formatters["terraform"]
	as.True(ok, "terraform formatter not found")
	as.Equal("terraform", terraform.Command)
	as.Equal([]string{"fmt"}, terraform.Options)
	as.Equal([]string{"*.tf"}, terraform.Includes)
	as.Nil(terraform.Excludes)

	// missing
	foo, ok := cfg.Formatters["foo-fmt"]
	as.True(ok, "foo formatter not found")
	as.Equal("foo-fmt", foo.Command)
}
