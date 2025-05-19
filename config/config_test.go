package config_test

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/numtide/treefmt/v2/config"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func newViper(t *testing.T) (*viper.Viper, *pflag.FlagSet) {
	t.Helper()

	v, err := config.NewViper()
	if err != nil {
		t.Fatal(err)
	}

	tempDir := t.TempDir()
	v.SetConfigFile(filepath.Join(tempDir, "treefmt.toml"))

	// initialise a git repo to help with tree-root-cmd testing
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir

	if err = cmd.Run(); err != nil {
		t.Fatal(err)
	}

	// change working directory to the temp dir
	t.Chdir(tempDir)

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	config.SetFlags(flags)

	if err := v.BindPFlags(flags); err != nil {
		t.Fatal(err)
	}

	return v, flags
}

func writeAndReadBack(t *testing.T, v *viper.Viper, cfg *config.Config) {
	t.Helper()

	// serialise the config and read it into viper
	buf := bytes.NewBuffer(nil)

	encoder := toml.NewEncoder(buf)
	if err := encoder.Encode(cfg); err != nil {
		t.Fatal(fmt.Errorf("failed to marshal config: %w", err))
	} else if err = v.ReadConfig(bufio.NewReader(buf)); err != nil {
		t.Fatal(fmt.Errorf("failed to read config: %w", err))
	}
}

func readError(t *testing.T, v *viper.Viper, cfg *config.Config, test func(error)) {
	t.Helper()

	writeAndReadBack(t, v, cfg)

	_, err := config.FromViper(v)
	if err == nil {
		t.Fatal("error was expected but none was thrown")
	}

	test(err)
}

func readValue(t *testing.T, v *viper.Viper, cfg *config.Config, test func(*config.Config)) {
	t.Helper()

	writeAndReadBack(t, v, cfg)

	//
	decodedCfg, err := config.FromViper(v)
	if err != nil {
		t.Fatal(fmt.Errorf("failed to unmarshal config from viper: %w", err))
	}

	test(decodedCfg)
}

func TestAllowMissingFormatter(t *testing.T) {
	as := require.New(t)

	cfg := &config.Config{}
	v, flags := newViper(t)

	checkValue := func(expected bool) {
		readValue(t, v, cfg, func(cfg *config.Config) {
			as.Equal(expected, cfg.AllowMissingFormatter)
		})
	}

	// default with no flag, env or config
	checkValue(false)

	// set config value
	cfg.AllowMissingFormatter = true
	checkValue(true)

	// env override
	t.Setenv("TREEFMT_ALLOW_MISSING_FORMATTER", "false")
	checkValue(false)

	// flag override
	as.NoError(flags.Set("allow-missing-formatter", "true"))
	checkValue(true)
}

func TestCI(t *testing.T) {
	as := require.New(t)

	cfg := &config.Config{}
	v, flags := newViper(t)

	checkValues := func(ci bool, noCache bool, failOnChange bool, verbosity uint8) {
		readValue(t, v, cfg, func(cfg *config.Config) {
			as.Equal(ci, cfg.CI)
			as.Equal(noCache, cfg.NoCache)
			as.Equal(failOnChange, cfg.FailOnChange)
			as.Equal(verbosity, cfg.Verbose)
		})
	}

	// default with no flag, env or config
	checkValues(false, false, false, 0)

	// set config value and check that it has no effect
	// you are not allowed to set ci in config
	cfg.CI = true

	checkValues(false, false, false, 0)

	// env override
	t.Setenv("TREEFMT_CI", "false")
	checkValues(false, false, false, 0)

	// flag override
	as.NoError(flags.Set("ci", "true"))
	checkValues(true, true, true, 1)

	// increase verbosity above 1 and check it isn't reset
	cfg.Verbose = 2

	checkValues(true, true, true, 2)
}

func TestClearCache(t *testing.T) {
	as := require.New(t)

	cfg := &config.Config{}
	v, flags := newViper(t)

	checkValue := func(expected bool) {
		readValue(t, v, cfg, func(cfg *config.Config) {
			as.Equal(expected, cfg.ClearCache)
		})
	}

	// default with no flag, env or config
	checkValue(false)

	// set config value and check that it has no effect
	// you are not allowed to set clear-cache in config
	cfg.ClearCache = true

	checkValue(false)

	// env override
	t.Setenv("TREEFMT_CLEAR_CACHE", "false")
	checkValue(false)

	// flag override
	as.NoError(flags.Set("clear-cache", "true"))
	checkValue(true)
}

func TestCpuProfile(t *testing.T) {
	as := require.New(t)

	cfg := &config.Config{}
	v, flags := newViper(t)

	checkValue := func(expected string) {
		readValue(t, v, cfg, func(cfg *config.Config) {
			as.Equal(expected, cfg.CPUProfile)
		})
	}

	// default with no flag, env or config
	checkValue("")

	// set config value
	cfg.CPUProfile = "/foo/bar"

	checkValue("/foo/bar")

	// env override
	t.Setenv("TREEFMT_CPU_PROFILE", "/fizz/buzz")
	checkValue("/fizz/buzz")

	// flag override
	as.NoError(flags.Set("cpu-profile", "/bla/bla"))
	checkValue("/bla/bla")
}

func TestExcludes(t *testing.T) {
	as := require.New(t)

	cfg := &config.Config{}
	v, flags := newViper(t)

	checkValue := func(expected []string) {
		readValue(t, v, cfg, func(cfg *config.Config) {
			as.Equal(expected, cfg.Excludes)
		})
	}

	// default with no env or config
	checkValue(nil)

	// set config value
	cfg.Excludes = []string{"foo", "bar"}

	checkValue([]string{"foo", "bar"})

	// test global.excludes fallback
	cfg.Excludes = nil
	cfg.Global.Excludes = []string{"fizz", "buzz"}

	checkValue([]string{"fizz", "buzz"})

	// env override
	t.Setenv("TREEFMT_EXCLUDES", "foo,bar")
	checkValue([]string{"foo", "bar"})

	// flag override
	as.NoError(flags.Set("excludes", "bleep,bloop"))
	checkValue([]string{"bleep", "bloop"})
}

func TestFailOnChange(t *testing.T) {
	as := require.New(t)

	cfg := &config.Config{}
	v, flags := newViper(t)

	checkValue := func(expected bool) {
		readValue(t, v, cfg, func(cfg *config.Config) {
			as.Equal(expected, cfg.FailOnChange)
		})
	}

	// default with no flag, env or config
	checkValue(false)

	// set config value
	cfg.FailOnChange = true
	checkValue(true)

	// env override
	t.Setenv("TREEFMT_FAIL_ON_CHANGE", "false")
	checkValue(false)

	// flag override
	as.NoError(flags.Set("fail-on-change", "true"))
	checkValue(true)
}

func TestFormatters(t *testing.T) {
	as := require.New(t)

	cfg := &config.Config{}
	v, flags := newViper(t)

	checkValue := func(expected []string) {
		readValue(t, v, cfg, func(cfg *config.Config) {
			as.Equal(expected, cfg.Formatters)
		})
	}

	// default with no env or config
	checkValue([]string{})

	// set config value
	cfg.FormatterConfigs = map[string]*config.Formatter{
		"echo": {
			Command: "echo",
		},
		"touch": {
			Command: "touch",
		},
		"date": {
			Command: "date",
		},
	}

	cfg.Formatters = []string{"echo", "touch"}

	checkValue([]string{"echo", "touch"})

	// env override
	t.Setenv("TREEFMT_FORMATTERS", "echo,date")
	checkValue([]string{"echo", "date"})

	// flag override
	as.NoError(flags.Set("formatters", "date,touch"))
	checkValue([]string{"date", "touch"})

	// bad formatter name
	as.NoError(flags.Set("formatters", "foo,echo,date"))

	_, err := config.FromViper(v)
	as.ErrorContains(err, "formatter foo not found in config")
}

func TestNoCache(t *testing.T) {
	as := require.New(t)

	cfg := &config.Config{}
	v, flags := newViper(t)

	checkValue := func(expected bool) {
		readValue(t, v, cfg, func(cfg *config.Config) {
			as.Equal(expected, cfg.NoCache)
		})
	}

	// default with no flag, env or config
	checkValue(false)

	// set config value and check that it has no effect
	// you are not allowed to set no-cache in config
	cfg.NoCache = true

	checkValue(false)

	// env override
	t.Setenv("TREEFMT_NO_CACHE", "false")
	checkValue(false)

	// flag override
	as.NoError(flags.Set("no-cache", "true"))
	checkValue(true)
}

func TestQuiet(t *testing.T) {
	as := require.New(t)

	cfg := &config.Config{}
	v, flags := newViper(t)

	checkValue := func(expected bool) {
		readValue(t, v, cfg, func(cfg *config.Config) {
			as.Equal(expected, cfg.Quiet)
		})
	}

	// default with no flag, env or config
	checkValue(false)

	// set config value and check that it has no effect
	// you are not allowed to set no-cache in config
	cfg.Quiet = true

	checkValue(false)

	// env override
	t.Setenv("TREEFMT_QUIET", "false")
	checkValue(false)

	// flag override
	as.NoError(flags.Set("quiet", "true"))
	checkValue(true)
}

func TestOnUnmatched(t *testing.T) {
	as := require.New(t)

	cfg := &config.Config{}
	v, flags := newViper(t)

	checkValue := func(expected string) {
		readValue(t, v, cfg, func(cfg *config.Config) {
			as.Equal(expected, cfg.OnUnmatched)
		})
	}

	// default with no flag, env or config
	checkValue("info")

	// set config value
	cfg.OnUnmatched = "error"

	checkValue("error")

	// env override
	t.Setenv("TREEFMT_ON_UNMATCHED", "debug")
	checkValue("debug")

	// flag override
	as.NoError(flags.Set("on-unmatched", "fatal"))
	checkValue("fatal")
}

func TestTreeRoot(t *testing.T) {
	as := require.New(t)

	cfg := &config.Config{}
	v, flags := newViper(t)

	checkValue := func(expected string) {
		readValue(t, v, cfg, func(cfg *config.Config) {
			as.Equal(expected, cfg.TreeRoot)
		})
	}

	// default with no flag, env or config
	// should match the absolute path of the directory in which the config file is located
	checkValue(filepath.Dir(v.ConfigFileUsed()))

	// set config value
	cfg.TreeRoot = "/foo/bar"

	checkValue("/foo/bar")

	// env override
	t.Setenv("TREEFMT_TREE_ROOT", "/fizz/buzz")
	checkValue("/fizz/buzz")

	// flag override
	as.NoError(flags.Set("tree-root", "/flip/flop"))
	checkValue("/flip/flop")
}

func TestTreeRootFile(t *testing.T) {
	as := require.New(t)

	cfg := &config.Config{}
	v, flags := newViper(t)

	// create a directory structure with config files at various levels
	tempDir := t.TempDir()
	as.NoError(os.MkdirAll(filepath.Join(tempDir, "foo", "bar"), 0o755))
	as.NoError(os.WriteFile(filepath.Join(tempDir, "foo", "bar", "a.txt"), []byte{}, 0o600))
	as.NoError(os.WriteFile(filepath.Join(tempDir, "foo", "go.mod"), []byte{}, 0o600))
	as.NoError(os.MkdirAll(filepath.Join(tempDir, ".git"), 0o755))
	as.NoError(os.WriteFile(filepath.Join(tempDir, ".git", "config"), []byte{}, 0o600))

	checkValue := func(treeRoot string, treeRootFile string) {
		readValue(t, v, cfg, func(cfg *config.Config) {
			as.Equal(treeRoot, cfg.TreeRoot)
			as.Equal(treeRootFile, cfg.TreeRootFile)
		})
	}

	// default with no flag, env or config
	// should match the absolute path of the directory in which the config file is located
	checkValue(filepath.Dir(v.ConfigFileUsed()), "")

	workDir := filepath.Join(tempDir, "foo", "bar")
	t.Setenv("TREEFMT_WORKING_DIR", workDir)

	// set config value
	// should match the lowest directory
	cfg.TreeRootFile = "a.txt"

	checkValue(workDir, "a.txt")

	// env override
	// should match the directory above
	t.Setenv("TREEFMT_TREE_ROOT_FILE", "go.mod")
	checkValue(filepath.Join(tempDir, "foo"), "go.mod")

	// flag override
	// should match the root of the temp directory structure
	as.NoError(flags.Set("tree-root-file", ".git/config"))
	checkValue(tempDir, ".git/config")
}

func TestTreeRootCmd(t *testing.T) {
	as := require.New(t)

	cfg := &config.Config{}
	v, flags := newViper(t)

	checkValue := func(treeRoot string) {
		readValue(t, v, cfg, func(cfg *config.Config) {
			as.Equal(treeRoot, cfg.TreeRoot)
		})
	}

	tempDir := t.TempDir()
	as.NoError(os.MkdirAll(filepath.Join(tempDir, "foo"), 0o755))
	as.NoError(os.MkdirAll(filepath.Join(tempDir, "bar"), 0o755))

	// default with no flag, env or config
	// should match the absolute path of the directory in which the config file is located
	checkValue(filepath.Dir(v.ConfigFileUsed()))

	// set config value
	cfg.TreeRootCmd = "echo " + tempDir
	checkValue(tempDir)

	// env override
	// should match the directory above
	t.Setenv("TREEFMT_TREE_ROOT_CMD", fmt.Sprintf("echo \"%s/foo\"", tempDir))
	checkValue(filepath.Join(tempDir, "foo"))

	// flag override
	// should match the root of the temp directory structure
	as.NoError(flags.Set("tree-root-cmd", fmt.Sprintf("echo '%s/bar'", tempDir)))
	checkValue(filepath.Join(tempDir, "bar"))

	// empty output from tree-root-cmd
	// should throw an error
	as.NoError(flags.Set("tree-root-cmd", "echo ''"))
	readError(t, v, cfg, func(err error) {
		as.ErrorContains(err, "empty output received after executing tree-root-cmd: echo ''")
	})
}

func TestVerbosity(t *testing.T) {
	as := require.New(t)

	cfg := &config.Config{}
	v, _ := newViper(t)

	checkValue := func(expected uint8) {
		readValue(t, v, cfg, func(cfg *config.Config) {
			as.Equal(expected, cfg.Verbose)
		})
	}

	// default with no flag, env or config
	checkValue(0)

	// set config value
	cfg.Verbose = 1

	checkValue(1)

	// flag override
	// todo unsure how to set a count flag via the flags api
	// as.NoError(flags.Set("verbose", "v"))
	// checkValue(1)

	// env override
	t.Setenv("TREEFMT_VERBOSE", "2")
	checkValue(2)
}

func TestWalk(t *testing.T) {
	as := require.New(t)

	cfg := &config.Config{}
	v, flags := newViper(t)

	checkValue := func(expected string) {
		readValue(t, v, cfg, func(cfg *config.Config) {
			as.Equal(expected, cfg.Walk)
		})
	}

	// default with no flag, env or config
	checkValue("auto")

	// set config value
	cfg.Walk = "git"

	checkValue("git")

	// env override
	t.Setenv("TREEFMT_WALK", "filesystem")
	checkValue("filesystem")

	// flag override
	as.NoError(flags.Set("walk", "auto"))
	checkValue("auto")
}

func TestWorkingDirectory(t *testing.T) {
	as := require.New(t)

	cfg := &config.Config{}
	v, flags := newViper(t)

	checkValue := func(expected string) {
		readValue(t, v, cfg, func(cfg *config.Config) {
			as.Equal(expected, cfg.WorkingDirectory)
		})
	}

	cwd, err := os.Getwd()
	as.NoError(err, "failed to get current working directory")
	cwd, err = filepath.Abs(cwd)
	as.NoError(err, "failed to get absolute path of current working directory")

	// default with no flag, env or config
	// current working directory by default
	checkValue(cwd)

	// set config value and check that it has no effect
	// you are not allowed to set working-dir in config
	cfg.WorkingDirectory = "/foo/bar/baz/../fizz"

	checkValue(cwd)

	// env override
	cwd = t.TempDir()
	t.Setenv("TREEFMT_WORKING_DIR", cwd+"/buzz/..")
	checkValue(cwd)

	// flag override
	cwd = t.TempDir()
	as.NoError(flags.Set("working-dir", cwd))
	checkValue(cwd)
}

func TestStdin(t *testing.T) {
	as := require.New(t)

	cfg := &config.Config{}
	v, flags := newViper(t)

	checkValues := func(stdin bool) {
		readValue(t, v, cfg, func(cfg *config.Config) {
			as.Equal(stdin, cfg.Stdin)
		})
	}

	// default with no flag, env or config
	checkValues(false)

	// set config value and check that it has no effect
	// you are not allowed to set stdin in config
	cfg.Stdin = true

	checkValues(false)

	// env override
	t.Setenv("TREEFMT_STDIN", "false")
	checkValues(false)

	// flag override
	as.NoError(flags.Set("stdin", "true"))
	checkValues(true)
}

func TestSampleConfigFile(t *testing.T) {
	as := require.New(t)

	v := viper.New()
	v.SetConfigFile("../test/examples/treefmt.toml")
	as.NoError(v.ReadInConfig(), "failed to read config file")

	cfg, err := config.FromViper(v)
	as.NoError(err, "failed to unmarshal config from viper")

	as.NotNil(cfg)
	as.Equal([]string{"*.toml"}, cfg.Excludes)

	// python
	python, ok := cfg.FormatterConfigs["python"]
	as.True(ok, "python formatter not found")
	as.Equal("black", python.Command)
	as.Nil(python.Options)
	as.Equal([]string{"*.py"}, python.Includes)
	as.Nil(python.Excludes)

	// go
	golang, ok := cfg.FormatterConfigs["go"]
	as.True(ok, "go formatter not found")
	as.Equal("gofmt", golang.Command)
	as.Equal([]string{"-w"}, golang.Options)
	as.Equal([]string{"*.go"}, golang.Includes)
	as.Nil(golang.Excludes)

	// haskell
	haskell, ok := cfg.FormatterConfigs["haskell"]
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
	alejandra, ok := cfg.FormatterConfigs["alejandra"]
	as.True(ok, "alejandra formatter not found")
	as.Equal("alejandra", alejandra.Command)
	as.Nil(alejandra.Options)
	as.Equal([]string{"*.nix"}, alejandra.Includes)
	as.Equal([]string{"examples/nix/sources.nix"}, alejandra.Excludes)
	as.Equal(1, alejandra.Priority)

	// deadnix
	deadnix, ok := cfg.FormatterConfigs["deadnix"]
	as.True(ok, "deadnix formatter not found")
	as.Equal("deadnix", deadnix.Command)
	as.Equal([]string{"-e"}, deadnix.Options)
	as.Equal([]string{"*.nix"}, deadnix.Includes)
	as.Nil(deadnix.Excludes)
	as.Equal(2, deadnix.Priority)

	// ruby
	ruby, ok := cfg.FormatterConfigs["ruby"]
	as.True(ok, "ruby formatter not found")
	as.Equal("rufo", ruby.Command)
	as.Equal([]string{"-x"}, ruby.Options)
	as.Equal([]string{"*.rb"}, ruby.Includes)
	as.Nil(ruby.Excludes)

	// prettier
	prettier, ok := cfg.FormatterConfigs["prettier"]
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
	rust, ok := cfg.FormatterConfigs["rust"]
	as.True(ok, "rust formatter not found")
	as.Equal("rustfmt", rust.Command)
	as.Equal([]string{"--edition", "2018"}, rust.Options)
	as.Equal([]string{"*.rs"}, rust.Includes)
	as.Nil(rust.Excludes)

	// shellcheck
	shellcheck, ok := cfg.FormatterConfigs["shellcheck"]
	as.True(ok, "shellcheck formatter not found")
	as.Equal("shellcheck", shellcheck.Command)
	as.Equal(1, shellcheck.Priority)
	as.Nil(shellcheck.Options)
	as.Equal([]string{"*.sh"}, shellcheck.Includes)
	as.Nil(shellcheck.Excludes)

	// shfmt
	shfmt, ok := cfg.FormatterConfigs["shfmt"]
	as.True(ok, "shfmt formatter not found")
	as.Equal("shfmt", shfmt.Command)
	as.Equal(2, shfmt.Priority)
	as.Equal([]string{"-i", "2", "-s", "-w"}, shfmt.Options)
	as.Equal([]string{"*.sh"}, shfmt.Includes)
	as.Nil(shfmt.Excludes)

	// opentofu
	opentofu, ok := cfg.FormatterConfigs["opentofu"]
	as.True(ok, "opentofu formatter not found")
	as.Equal("tofu", opentofu.Command)
	as.Equal([]string{"fmt"}, opentofu.Options)
	as.Equal([]string{"*.tf"}, opentofu.Includes)
	as.Nil(opentofu.Excludes)

	// missing
	foo, ok := cfg.FormatterConfigs["foo-fmt"]
	as.True(ok, "foo formatter not found")
	as.Equal("foo-fmt", foo.Command)
}
