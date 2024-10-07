package config

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/stretchr/testify/require"
)

func newViper(t *testing.T) (*viper.Viper, *pflag.FlagSet) {
	t.Helper()
	v := NewViper()

	tempDir := t.TempDir()
	v.SetConfigFile(filepath.Join(tempDir, "treefmt.toml"))

	flags := SetFlags(pflag.NewFlagSet("test", pflag.ContinueOnError))
	if err := v.BindPFlags(flags); err != nil {
		t.Fatal(err)
	}
	return v, flags
}

func readValue(t *testing.T, v *viper.Viper, cfg map[string]any, test func(*Config)) {
	t.Helper()

	// serialise the config and read it into viper
	buf := bytes.NewBuffer(nil)
	encoder := toml.NewEncoder(buf)
	if err := encoder.Encode(cfg); err != nil {
		t.Fatal(fmt.Errorf("failed to marshal config: %w", err))
	} else if err = v.ReadConfig(bufio.NewReader(buf)); err != nil {
		t.Fatal(fmt.Errorf("failed to read config: %w", err))
	}

	//
	decodedCfg, err := FromViper(v)
	if err != nil {
		t.Fatal(fmt.Errorf("failed to unmarshal config from viper: %w", err))
	}

	test(decodedCfg)
}

func TestAllowMissingFormatter(t *testing.T) {
	as := require.New(t)

	cfg := make(map[string]any)
	v, flags := newViper(t)

	checkValue := func(expected bool) {
		readValue(t, v, cfg, func(cfg *Config) {
			as.Equal(expected, cfg.AllowMissingFormatter)
		})
	}

	// default with no flag, env or config
	checkValue(false)

	// set config value
	cfg["allow-missing-formatter"] = true
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

	cfg := make(map[string]any)
	v, flags := newViper(t)

	checkValues := func(ci bool, noCache bool, failOnChange bool, verbosity uint8) {
		readValue(t, v, cfg, func(cfg *Config) {
			as.Equal(ci, cfg.CI)
			as.Equal(noCache, cfg.NoCache)
			as.Equal(failOnChange, cfg.FailOnChange)
			as.Equal(verbosity, cfg.Verbosity)
		})
	}

	// default with no flag, env or config
	checkValues(false, false, false, 0)

	// set config value
	cfg["ci"] = true
	checkValues(true, true, true, 1)

	// env override
	t.Setenv("TREEFMT_CI", "false")
	checkValues(false, false, false, 0)

	// flag override
	as.NoError(flags.Set("ci", "true"))
	checkValues(true, true, true, 1)

	// increase verbosity above 1 and check it isn't reset
	cfg["verbose"] = 2
	checkValues(true, true, true, 2)
}

func TestClearCache(t *testing.T) {
	as := require.New(t)

	cfg := make(map[string]any)
	v, flags := newViper(t)

	checkValue := func(expected bool) {
		readValue(t, v, cfg, func(cfg *Config) {
			as.Equal(expected, cfg.ClearCache)
		})
	}

	// default with no flag, env or config
	checkValue(false)

	// set config value
	cfg["clear-cache"] = true
	checkValue(true)

	// env override
	t.Setenv("TREEFMT_CLEAR_CACHE", "false")
	checkValue(false)

	// flag override
	as.NoError(flags.Set("clear-cache", "true"))
	checkValue(true)
}

func TestCpuProfile(t *testing.T) {
	as := require.New(t)

	cfg := make(map[string]any)
	v, flags := newViper(t)

	checkValue := func(expected string) {
		readValue(t, v, cfg, func(cfg *Config) {
			as.Equal(expected, cfg.CpuProfile)
		})
	}

	// default with no flag, env or config
	checkValue("")

	// set config value
	cfg["cpu-profile"] = "/foo/bar"
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

	cfg := make(map[string]any)
	v, flags := newViper(t)

	checkValue := func(expected []string) {
		readValue(t, v, cfg, func(cfg *Config) {
			as.Equal(expected, cfg.Excludes)
		})
	}

	// default with no env or config
	checkValue(nil)

	// set config value
	cfg["excludes"] = []string{"foo", "bar"}
	checkValue([]string{"foo", "bar"})

	// test global.excludes fallback
	delete(cfg, "excludes")
	cfg["global"] = map[string]any{
		"excludes": []string{"fizz", "buzz"},
	}
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

	cfg := make(map[string]any)
	v, flags := newViper(t)

	checkValue := func(expected bool) {
		readValue(t, v, cfg, func(cfg *Config) {
			as.Equal(expected, cfg.FailOnChange)
		})
	}

	// default with no flag, env or config
	checkValue(false)

	// set config value
	cfg["fail-on-change"] = true
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

	cfg := make(map[string]any)
	v, flags := newViper(t)

	checkValue := func(expected []string) {
		readValue(t, v, cfg, func(cfg *Config) {
			as.Equal(expected, cfg.Formatters)
		})
	}

	// default with no env or config
	checkValue([]string{})

	// set config value
	cfg["formatter"] = map[string]any{
		"echo": map[string]any{
			"command": "echo",
		},
		"touch": map[string]any{
			"command": "touch",
		},
		"date": map[string]any{
			"command": "date",
		},
	}
	cfg["formatters"] = []string{"echo", "touch"}
	checkValue([]string{"echo", "touch"})

	// env override
	t.Setenv("TREEFMT_FORMATTERS", "echo,date")
	checkValue([]string{"echo", "date"})

	// flag override
	as.NoError(flags.Set("formatters", "date,touch"))
	checkValue([]string{"date", "touch"})

	// bad formatter name
	as.NoError(flags.Set("formatters", "foo,echo,date"))
	_, err := FromViper(v)
	as.ErrorContains(err, "formatter foo not found in config")
}

func TestNoCache(t *testing.T) {
	as := require.New(t)

	cfg := make(map[string]any)
	v, flags := newViper(t)

	checkValue := func(expected bool) {
		readValue(t, v, cfg, func(cfg *Config) {
			as.Equal(expected, cfg.NoCache)
		})
	}

	// default with no flag, env or config
	checkValue(false)

	// set config value
	cfg["no-cache"] = true
	checkValue(true)

	// env override
	t.Setenv("TREEFMT_NO_CACHE", "false")
	checkValue(false)

	// flag override
	as.NoError(flags.Set("no-cache", "true"))
	checkValue(true)
}

func TestOnUnmatched(t *testing.T) {
	as := require.New(t)

	cfg := make(map[string]any)
	v, flags := newViper(t)

	checkValue := func(expected string) {
		readValue(t, v, cfg, func(cfg *Config) {
			as.Equal(expected, cfg.OnUnmatched)
		})
	}

	// default with no flag, env or config
	checkValue("warn")

	// set config value
	cfg["on-unmatched"] = "error"
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

	cfg := make(map[string]any)
	v, flags := newViper(t)

	checkValue := func(expected string) {
		readValue(t, v, cfg, func(cfg *Config) {
			as.Equal(expected, cfg.TreeRoot)
		})
	}

	// default with no flag, env or config
	// should match the absolute path of the directory in which the config file is located
	checkValue(filepath.Dir(v.ConfigFileUsed()))

	// set config value
	cfg["tree-root"] = "/foo/bar"
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

	cfg := make(map[string]any)
	v, flags := newViper(t)

	// create a directory structure with config files at various levels
	tempDir := t.TempDir()
	as.NoError(os.MkdirAll(filepath.Join(tempDir, "foo", "bar"), 0o755))
	as.NoError(os.WriteFile(filepath.Join(tempDir, "foo", "bar", "a.txt"), []byte{}, 0o644))
	as.NoError(os.WriteFile(filepath.Join(tempDir, "foo", "go.mod"), []byte{}, 0o644))
	as.NoError(os.MkdirAll(filepath.Join(tempDir, ".git"), 0o755))
	as.NoError(os.WriteFile(filepath.Join(tempDir, ".git", "config"), []byte{}, 0o644))

	checkValue := func(treeRoot string, treeRootFile string) {
		readValue(t, v, cfg, func(cfg *Config) {
			as.Equal(treeRoot, cfg.TreeRoot)
			as.Equal(treeRootFile, cfg.TreeRootFile)
		})
	}

	// default with no flag, env or config
	// should match the absolute path of the directory in which the config file is located
	checkValue(filepath.Dir(v.ConfigFileUsed()), "")

	// set config value
	// should match the lowest directory
	workDir := filepath.Join(tempDir, "foo", "bar")
	cfg["working-dir"] = workDir
	cfg["tree-root-file"] = "a.txt"
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

func TestVerbosity(t *testing.T) {
	as := require.New(t)

	cfg := make(map[string]any)
	v, _ := newViper(t)

	checkValue := func(expected uint8) {
		readValue(t, v, cfg, func(cfg *Config) {
			as.Equal(expected, cfg.Verbosity)
		})
	}

	// default with no flag, env or config
	checkValue(0)

	// set config value
	cfg["verbose"] = 1
	checkValue(1)

	// env override
	t.Setenv("TREEFMT_VERBOSE", "2")
	checkValue(2)

	// flag override
	// todo unsure how to set a count flag via the flags api
	// as.NoError(flags.Set("verbose", "v"))
	// checkValue(1)
}

func TestWalk(t *testing.T) {
	as := require.New(t)

	cfg := make(map[string]any)
	v, flags := newViper(t)

	checkValue := func(expected string) {
		readValue(t, v, cfg, func(cfg *Config) {
			as.Equal(expected, cfg.Walk)
		})
	}

	// default with no flag, env or config
	checkValue("auto")

	// set config value
	cfg["walk"] = "git"
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

	cfg := make(map[string]any)
	v, flags := newViper(t)

	checkValue := func(expected string) {
		readValue(t, v, cfg, func(cfg *Config) {
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

	// set config value
	// should resolve input paths to absolute paths
	cfg["working-dir"] = "/foo/bar/baz/../fizz"
	checkValue("/foo/bar/fizz")

	// env override
	t.Setenv("TREEFMT_WORKING_DIR", "/fizz/buzz/..")
	checkValue("/fizz")

	// flag override
	as.NoError(flags.Set("working-dir", "/flip/flop"))
	checkValue("/flip/flop")
}

func TestStdin(t *testing.T) {
	as := require.New(t)

	cfg := make(map[string]any)
	v, flags := newViper(t)

	checkValues := func(stdin bool) {
		readValue(t, v, cfg, func(cfg *Config) {
			as.Equal(stdin, cfg.Stdin)
		})
	}

	// default with no flag, env or config
	checkValues(false)

	// set config value
	cfg["stdin"] = true
	checkValues(true)

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

	cfg, err := FromViper(v)
	as.NoError(err, "failed to unmarshal config from viper")

	as.NotNil(cfg)
	as.Equal([]string{"*.toml"}, cfg.Global.Excludes)

	// python
	python, ok := cfg.FormatterConfigs["python"]
	as.True(ok, "python formatter not found")
	as.Equal("black", python.Command)
	as.Nil(python.Options)
	as.Equal([]string{"*.py"}, python.Includes)
	as.Nil(python.Excludes)

	// elm
	elm, ok := cfg.FormatterConfigs["elm"]
	as.True(ok, "elm formatter not found")
	as.Equal("elm-format", elm.Command)
	as.Equal([]string{"--yes"}, elm.Options)
	as.Equal([]string{"*.elm"}, elm.Includes)
	as.Nil(elm.Excludes)

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
	as.Equal(shfmt.Options, []string{"-i", "2", "-s", "-w"})
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
