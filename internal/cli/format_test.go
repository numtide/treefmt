package cli

import (
	"fmt"
	"os"
	"os/exec"
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

	out, err := cmd(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	as.Contains(string(out), "3 files changed")

	out, err = cmd(t, "-c", "--config-file", configPath, "--tree-root", tempDir, "--formatters", "elm,nix")
	as.NoError(err)
	as.Contains(string(out), "2 files changed")

	out, err = cmd(t, "-c", "--config-file", configPath, "--tree-root", tempDir, "--formatters", "ruby,nix")
	as.NoError(err)
	as.Contains(string(out), "2 files changed")

	out, err = cmd(t, "-c", "--config-file", configPath, "--tree-root", tempDir, "--formatters", "nix")
	as.NoError(err)
	as.Contains(string(out), "1 files changed")

	// test bad names

	out, err = cmd(t, "-c", "--config-file", configPath, "--tree-root", tempDir, "--formatters", "foo")
	as.Errorf(err, "formatter not found in config: foo")

	out, err = cmd(t, "-c", "--config-file", configPath, "--tree-root", tempDir, "--formatters", "bar,foo")
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
	config.Global.Excludes = []string{"*.nix"}

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

	echo := config.Formatters["echo"]

	// remove python files from the echo formatter
	echo.Excludes = []string{"*.py"}

	test.WriteConfig(t, configPath, config)
	out, err = cmd(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	as.Contains(string(out), fmt.Sprintf("%d files changed", 20))

	// remove go files from the echo formatter
	echo.Excludes = []string{"*.py", "*.go"}

	test.WriteConfig(t, configPath, config)
	out, err = cmd(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	as.Contains(string(out), fmt.Sprintf("%d files changed", 19))

	// adjust the includes for echo to only include elm files
	echo.Includes = []string{"*.elm"}

	test.WriteConfig(t, configPath, config)
	out, err = cmd(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	as.Contains(string(out), fmt.Sprintf("%d files changed", 1))

	// add js files to echo formatter
	echo.Includes = []string{"*.elm", "*.js"}

	test.WriteConfig(t, configPath, config)
	out, err = cmd(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	as.Contains(string(out), fmt.Sprintf("%d files changed", 2))
}

func TestCache(t *testing.T) {
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
	out, err := cmd(t, "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	as.Contains(string(out), fmt.Sprintf("%d files changed", 29))

	out, err = cmd(t, "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	as.Contains(string(out), "0 files changed")
}

func TestChangeWorkingDirectory(t *testing.T) {
	as := require.New(t)

	// capture current cwd, so we can replace it after the test is finished
	cwd, err := os.Getwd()
	as.NoError(err)

	t.Cleanup(func() {
		// return to the previous working directory
		as.NoError(os.Chdir(cwd))
	})

	tempDir := test.TempExamples(t)
	configPath := tempDir + "/treefmt.toml"

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

	// by default, we look for ./treefmt.toml and use the cwd for the tree root
	// this should fail if the working directory hasn't been changed first
	out, err := cmd(t, "-C", tempDir)
	as.NoError(err)
	as.Contains(string(out), fmt.Sprintf("%d files changed", 29))
}

func TestFailOnChange(t *testing.T) {
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
	_, err := cmd(t, "--fail-on-change", "--config-file", configPath, "--tree-root", tempDir)
	as.ErrorIs(err, ErrFailOnChange)
}

func TestBustCacheOnFormatterChange(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)
	configPath := tempDir + "/echo.toml"

	// symlink some formatters into temp dir, so we can mess with their mod times
	binPath := tempDir + "/bin"
	as.NoError(os.Mkdir(binPath, 0o755))

	binaries := []string{"black", "elm-format", "gofmt"}

	for _, name := range binaries {
		src, err := exec.LookPath(name)
		as.NoError(err)
		as.NoError(os.Symlink(src, binPath+"/"+name))
	}

	// prepend our test bin directory to PATH
	as.NoError(os.Setenv("PATH", binPath+":"+os.Getenv("PATH")))

	// start with 2 formatters
	config := format.Config{
		Formatters: map[string]*format.Formatter{
			"python": {
				Command:  "black",
				Includes: []string{"*.py"},
			},
			"elm": {
				Command:  "elm-format",
				Options:  []string{"--yes"},
				Includes: []string{"*.elm"},
			},
		},
	}

	test.WriteConfig(t, configPath, config)
	args := []string{"--config-file", configPath, "--tree-root", tempDir}
	out, err := cmd(t, args...)
	as.NoError(err)
	as.Contains(string(out), fmt.Sprintf("%d files changed", 3))

	// tweak mod time of elm formatter
	as.NoError(test.RecreateSymlink(t, binPath+"/"+"elm-format"))

	out, err = cmd(t, args...)
	as.NoError(err)
	as.Contains(string(out), fmt.Sprintf("%d files changed", 3))

	// check cache is working
	out, err = cmd(t, args...)
	as.NoError(err)
	as.Contains(string(out), "0 files changed")

	// tweak mod time of python formatter
	as.NoError(test.RecreateSymlink(t, binPath+"/"+"black"))

	out, err = cmd(t, args...)
	as.NoError(err)
	as.Contains(string(out), fmt.Sprintf("%d files changed", 3))

	// check cache is working
	out, err = cmd(t, args...)
	as.NoError(err)
	as.Contains(string(out), "0 files changed")

	// add go formatter
	config.Formatters["go"] = &format.Formatter{
		Command:  "gofmt",
		Options:  []string{"-w"},
		Includes: []string{"*.go"},
	}
	test.WriteConfig(t, configPath, config)

	out, err = cmd(t, args...)
	as.NoError(err)
	as.Contains(string(out), fmt.Sprintf("%d files changed", 4))

	// check cache is working
	out, err = cmd(t, args...)
	as.NoError(err)
	as.Contains(string(out), "0 files changed")

	// remove python formatter
	delete(config.Formatters, "python")
	test.WriteConfig(t, configPath, config)

	out, err = cmd(t, args...)
	as.NoError(err)
	as.Contains(string(out), fmt.Sprintf("%d files changed", 2))

	// check cache is working
	out, err = cmd(t, args...)
	as.NoError(err)
	as.Contains(string(out), "0 files changed")

	// remove elm formatter
	delete(config.Formatters, "elm")
	test.WriteConfig(t, configPath, config)

	out, err = cmd(t, args...)
	as.NoError(err)
	as.Contains(string(out), fmt.Sprintf("%d files changed", 1))

	// check cache is working
	out, err = cmd(t, args...)
	as.NoError(err)
	as.Contains(string(out), "0 files changed")
}
