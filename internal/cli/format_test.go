package cli

import (
	"fmt"
	"testing"

	"git.numtide.com/numtide/treefmt/internal/test"

	"git.numtide.com/numtide/treefmt/internal/format"
	"github.com/stretchr/testify/require"
)

func TestAllowMissingFormatter(t *testing.T) {
	as := require.New(t)

	tempDir := t.TempDir()
	configPath := tempDir + "/treefmt.toml"

	test.WriteConfig(t, configPath, format.Config{
		Formatters: map[string]*format.Formatter{
			"foo-fmt": {
				Command: "foo-fmt",
			},
		},
	})

	_, err := cmd(t, "--config-file", configPath, "--tree-root", tempDir)
	as.ErrorIs(err, format.ErrFormatterNotFound)

	_, err = cmd(t, "--config-file", configPath, "--tree-root", tempDir, "--allow-missing-formatter")
	as.NoError(err)
}

func TestSpecifyingFormatters(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)
	configPath := tempDir + "/treefmt.toml"

	test.WriteConfig(t, configPath, format.Config{
		Formatters: map[string]*format.Formatter{
			"elm": {
				Command:  "echo",
				Includes: []string{"*.elm"},
			},
			"nix": {
				Command:  "echo",
				Includes: []string{"*.nix"},
			},
			"ruby": {
				Command:  "echo",
				Includes: []string{"*.rb"},
			},
		},
	})

	out, err := cmd(t, "--clear-cache", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	as.Contains(string(out), "3 files changed")

	out, err = cmd(t, "--clear-cache", "--config-file", configPath, "--tree-root", tempDir, "--formatters", "elm,nix")
	as.NoError(err)
	as.Contains(string(out), "2 files changed")

	out, err = cmd(t, "--clear-cache", "--config-file", configPath, "--tree-root", tempDir, "--formatters", "ruby,nix")
	as.NoError(err)
	as.Contains(string(out), "2 files changed")

	out, err = cmd(t, "--clear-cache", "--config-file", configPath, "--tree-root", tempDir, "--formatters", "nix")
	as.NoError(err)
	as.Contains(string(out), "1 files changed")

	// test bad names

	out, err = cmd(t, "--clear-cache", "--config-file", configPath, "--tree-root", tempDir, "--formatters", "foo")
	as.Errorf(err, "formatter not found in config: foo")

	out, err = cmd(t, "--clear-cache", "--config-file", configPath, "--tree-root", tempDir, "--formatters", "bar,foo")
	as.Errorf(err, "formatter not found in config: bar")
}

func TestIncludesAndExcludes(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)
	configPath := tempDir + "/echo.toml"

	// test without any excludes
	config := format.Config{
		Formatters: map[string]*format.Formatter{
			"echo": {
				Command:  "echo",
				Includes: []string{"*"},
			},
		},
	}

	test.WriteConfig(t, configPath, config)
	out, err := cmd(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	as.Contains(string(out), fmt.Sprintf("%d files changed", 29))

	// globally exclude nix files
	config.Global = struct{ Excludes []string }{
		Excludes: []string{"*.nix"},
	}

	test.WriteConfig(t, configPath, config)
	out, err = cmd(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	as.Contains(string(out), fmt.Sprintf("%d files changed", 28))

	// add haskell files to the global exclude
	config.Global.Excludes = []string{"*.nix", "*.hs"}

	test.WriteConfig(t, configPath, config)
	out, err = cmd(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	as.Contains(string(out), fmt.Sprintf("%d files changed", 22))

	// remove python files from the echo formatter
	config.Formatters["echo"].Excludes = []string{"*.py"}

	test.WriteConfig(t, configPath, config)
	out, err = cmd(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	as.Contains(string(out), fmt.Sprintf("%d files changed", 20))

	// remove go files from the echo formatter
	config.Formatters["echo"].Excludes = []string{"*.py", "*.go"}

	test.WriteConfig(t, configPath, config)
	out, err = cmd(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	as.Contains(string(out), fmt.Sprintf("%d files changed", 19))

	// adjust the includes for echo to only include elm files
	config.Formatters["echo"].Includes = []string{"*.elm"}

	test.WriteConfig(t, configPath, config)
	out, err = cmd(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	as.Contains(string(out), fmt.Sprintf("%d files changed", 1))

	// add js files to echo formatter
	config.Formatters["echo"].Includes = []string{"*.elm", "*.js"}

	test.WriteConfig(t, configPath, config)
	out, err = cmd(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	as.Contains(string(out), fmt.Sprintf("%d files changed", 2))
}
