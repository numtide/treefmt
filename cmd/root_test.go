package cmd_test

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/numtide/treefmt/cmd"
	formatCmd "github.com/numtide/treefmt/cmd/format"
	"github.com/numtide/treefmt/config"
	"github.com/numtide/treefmt/format"
	"github.com/numtide/treefmt/stats"
	"github.com/numtide/treefmt/test"
	"github.com/numtide/treefmt/walk"
	"github.com/stretchr/testify/require"
)

func TestOnUnmatched(t *testing.T) {
	as := require.New(t)

	// capture current cwd, so we can replace it after the test is finished
	cwd, err := os.Getwd()
	as.NoError(err)

	t.Cleanup(func() {
		// return to the previous working directory
		as.NoError(os.Chdir(cwd))
	})

	tempDir := test.TempExamples(t)

	paths := []string{
		"go/go.mod",
		"haskell/haskell.cabal",
		"html/scripts/.gitkeep",
		"python/requirements.txt",
		// these should not be reported as they're in the global excludes
		// - "nixpkgs.toml"
		// - "touch.toml"
		// - "treefmt.toml"
		// - "rust/Cargo.toml"
		// - "haskell/treefmt.toml"
	}

	_, _, err = treefmt(t, "-C", tempDir, "--allow-missing-formatter", "--on-unmatched", "fatal")
	as.ErrorContains(err, fmt.Sprintf("no formatter for path: %s", paths[0]))

	checkOutput := func(level string, output []byte) {
		for _, p := range paths {
			as.Contains(string(output), fmt.Sprintf("%s no formatter for path: %s", level, p))
		}
	}

	var out []byte

	// default is warn
	out, _, err = treefmt(t, "-C", tempDir, "--allow-missing-formatter")
	as.NoError(err)
	checkOutput("WARN", out)

	out, _, err = treefmt(t, "-C", tempDir, "--allow-missing-formatter", "--on-unmatched", "warn")
	as.NoError(err)
	checkOutput("WARN", out)

	out, _, err = treefmt(t, "-C", tempDir, "--allow-missing-formatter", "-u", "error")
	as.NoError(err)
	checkOutput("ERRO", out)

	out, _, err = treefmt(t, "-C", tempDir, "--allow-missing-formatter", "-v", "--on-unmatched", "info")
	as.NoError(err)
	checkOutput("INFO", out)

	t.Setenv("TREEFMT_ON_UNMATCHED", "debug")
	out, _, err = treefmt(t, "-C", tempDir, "--allow-missing-formatter", "-vv")
	as.NoError(err)
	checkOutput("DEBU", out)
}

func TestCpuProfile(t *testing.T) {
	as := require.New(t)
	tempDir := test.TempExamples(t)

	// capture current cwd, so we can replace it after the test is finished
	cwd, err := os.Getwd()
	as.NoError(err)

	t.Cleanup(func() {
		// return to the previous working directory
		as.NoError(os.Chdir(cwd))
	})

	_, _, err = treefmt(t, "-C", tempDir, "--allow-missing-formatter", "--cpu-profile", "cpu.pprof")
	as.NoError(err)
	as.FileExists(filepath.Join(tempDir, "cpu.pprof"))
	_, err = os.Stat(filepath.Join(tempDir, "cpu.pprof"))
	as.NoError(err)

	t.Setenv("TREEFMT_CPU_PROFILE", "env.pprof")
	_, _, err = treefmt(t, "-C", tempDir, "--allow-missing-formatter")
	as.NoError(err)
	as.FileExists(filepath.Join(tempDir, "env.pprof"))
	_, err = os.Stat(filepath.Join(tempDir, "env.pprof"))
	as.NoError(err)
}

func TestAllowMissingFormatter(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)
	configPath := tempDir + "/treefmt.toml"

	test.WriteConfig(t, configPath, &config.Config{
		FormatterConfigs: map[string]*config.Formatter{
			"foo-fmt": {
				Command: "foo-fmt",
			},
		},
	})

	_, _, err := treefmt(t, "--config-file", configPath, "--tree-root", tempDir, "-vv")
	as.ErrorIs(err, format.ErrCommandNotFound)

	_, _, err = treefmt(t, "--config-file", configPath, "--tree-root", tempDir, "--allow-missing-formatter")
	as.NoError(err)

	t.Setenv("TREEFMT_ALLOW_MISSING_FORMATTER", "true")
	_, _, err = treefmt(t, "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
}

func TestSpecifyingFormatters(t *testing.T) {
	as := require.New(t)

	cfg := &config.Config{
		FormatterConfigs: map[string]*config.Formatter{
			"elm": {
				Command:  "touch",
				Options:  []string{"-m"},
				Includes: []string{"*.elm"},
			},
			"nix": {
				Command:  "touch",
				Options:  []string{"-m"},
				Includes: []string{"*.nix"},
			},
			"ruby": {
				Command:  "touch",
				Options:  []string{"-m"},
				Includes: []string{"*.rb"},
			},
		},
	}

	var tempDir, configPath string

	// we reset the temp dir between successive runs as it appears that touching the file and modifying the mtime can
	// is not granular enough between assertions in quick succession
	setup := func() {
		tempDir = test.TempExamples(t)
		configPath = tempDir + "/treefmt.toml"
		test.WriteConfig(t, configPath, cfg)
	}

	setup()

	_, statz, err := treefmt(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   3,
		stats.Formatted: 3,
		stats.Changed:   3,
	})

	setup()

	_, statz, err = treefmt(t, "-c", "--config-file", configPath, "--tree-root", tempDir, "--formatters", "elm,nix")
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   2,
		stats.Formatted: 2,
		stats.Changed:   2,
	})

	setup()

	_, statz, err = treefmt(t, "-c", "--config-file", configPath, "--tree-root", tempDir, "-f", "ruby,nix")
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   2,
		stats.Formatted: 2,
		stats.Changed:   2,
	})

	setup()

	_, statz, err = treefmt(t, "-c", "--config-file", configPath, "--tree-root", tempDir, "--formatters", "nix")
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   1,
		stats.Formatted: 1,
		stats.Changed:   1,
	})

	// test bad names
	setup()

	_, _, err = treefmt(t, "-c", "--config-file", configPath, "--tree-root", tempDir, "--formatters", "foo")
	as.Errorf(err, "formatter not found in config: foo")

	t.Setenv("TREEFMT_FORMATTERS", "bar,foo")
	_, _, err = treefmt(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.Errorf(err, "formatter not found in config: bar")
}

func TestIncludesAndExcludes(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)
	configPath := tempDir + "/touch.toml"

	// test without any excludes
	cfg := &config.Config{
		FormatterConfigs: map[string]*config.Formatter{
			"echo": {
				Command:  "echo",
				Includes: []string{"*"},
			},
		},
	}

	test.WriteConfig(t, configPath, cfg)
	_, statz, err := treefmt(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   32,
		stats.Formatted: 32,
		stats.Changed:   0,
	})

	// globally exclude nix files
	cfg.Excludes = []string{"*.nix"}

	test.WriteConfig(t, configPath, cfg)
	_, statz, err = treefmt(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   31,
		stats.Formatted: 31,
		stats.Changed:   0,
	})

	// add haskell files to the global exclude
	cfg.Excludes = []string{"*.nix", "*.hs"}

	test.WriteConfig(t, configPath, cfg)
	_, statz, err = treefmt(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   25,
		stats.Formatted: 25,
		stats.Changed:   0,
	})

	echo := cfg.FormatterConfigs["echo"]

	// remove python files from the echo formatter
	echo.Excludes = []string{"*.py"}

	test.WriteConfig(t, configPath, cfg)
	_, statz, err = treefmt(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   23,
		stats.Formatted: 23,
		stats.Changed:   0,
	})

	// remove go files from the echo formatter via env
	t.Setenv("TREEFMT_FORMATTER_ECHO_EXCLUDES", "*.py,*.go")

	test.WriteConfig(t, configPath, cfg)
	_, statz, err = treefmt(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   22,
		stats.Formatted: 22,
		stats.Changed:   0,
	})

	t.Setenv("TREEFMT_FORMATTER_ECHO_EXCLUDES", "") // reset

	// adjust the includes for echo to only include elm files
	echo.Includes = []string{"*.elm"}

	test.WriteConfig(t, configPath, cfg)
	_, statz, err = treefmt(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   1,
		stats.Formatted: 1,
		stats.Changed:   0,
	})

	// add js files to echo formatter via env
	t.Setenv("TREEFMT_FORMATTER_ECHO_INCLUDES", "*.elm,*.js")

	test.WriteConfig(t, configPath, cfg)
	_, statz, err = treefmt(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   2,
		stats.Formatted: 2,
		stats.Changed:   0,
	})
}

func TestPrjRootEnvVariable(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)
	configPath := tempDir + "/treefmt.toml"

	// test without any excludes
	cfg := &config.Config{
		FormatterConfigs: map[string]*config.Formatter{
			"echo": {
				Command:  "echo",
				Includes: []string{"*"},
			},
		},
	}

	test.WriteConfig(t, configPath, cfg)
	t.Setenv("PRJ_ROOT", tempDir)
	_, statz, err := treefmt(t, "--config-file", configPath)
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   32,
		stats.Formatted: 32,
		stats.Changed:   0,
	})
}

func TestCache(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)
	configPath := tempDir + "/touch.toml"

	// test without any excludes
	cfg := &config.Config{
		FormatterConfigs: map[string]*config.Formatter{
			"echo": {
				Command:  "echo",
				Includes: []string{"*"},
			},
		},
	}

	var err error

	test.WriteConfig(t, configPath, cfg)
	_, statz, err := treefmt(t, "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   32,
		stats.Formatted: 32,
		stats.Changed:   0,
	})

	_, statz, err = treefmt(t, "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   32,
		stats.Formatted: 0,
		stats.Changed:   0,
	})

	// clear cache
	_, statz, err = treefmt(t, "--config-file", configPath, "--tree-root", tempDir, "-c")
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   32,
		stats.Formatted: 32,
		stats.Changed:   0,
	})

	_, statz, err = treefmt(t, "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   32,
		stats.Formatted: 0,
		stats.Changed:   0,
	})

	// clear cache
	_, statz, err = treefmt(t, "--config-file", configPath, "--tree-root", tempDir, "-c")
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   32,
		stats.Formatted: 32,
		stats.Changed:   0,
	})

	_, statz, err = treefmt(t, "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   32,
		stats.Formatted: 0,
		stats.Changed:   0,
	})

	// no cache
	_, statz, err = treefmt(t, "--config-file", configPath, "--tree-root", tempDir, "--no-cache")
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   32,
		stats.Formatted: 32,
		stats.Changed:   0,
	})

	// test that formatting errors are not cached

	// update the config with a failing formatter
	cfg = &config.Config{
		FormatterConfigs: map[string]*config.Formatter{
			// fails to execute
			"fail": {
				Command:  "touch",
				Options:  []string{"--bad-arg"},
				Includes: []string{"*.hs"},
			},
		},
	}
	test.WriteConfig(t, configPath, cfg)

	// running should match but not format anything
	_, statz, err = treefmt(t, "--config-file", configPath, "--tree-root", tempDir)
	as.ErrorIs(err, format.ErrFormattingFailures)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   6,
		stats.Formatted: 0,
		stats.Changed:   0,
	})

	// running again should provide the same result
	_, statz, err = treefmt(t, "--config-file", configPath, "--tree-root", tempDir)
	as.ErrorIs(err, format.ErrFormattingFailures)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   6,
		stats.Formatted: 0,
		stats.Changed:   0,
	})

	// let's fix the haskell config so it no longer fails
	cfg.FormatterConfigs["fail"].Options = nil
	test.WriteConfig(t, configPath, cfg)

	// we should now format the haskell files
	_, statz, err = treefmt(t, "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   6,
		stats.Formatted: 6,
		stats.Changed:   6,
	})
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
	cfg := &config.Config{
		FormatterConfigs: map[string]*config.Formatter{
			"echo": {
				Command:  "echo",
				Includes: []string{"*"},
			},
		},
	}

	test.WriteConfig(t, configPath, cfg)

	// by default, we look for ./treefmt.toml and use the cwd for the tree root
	// this should fail if the working directory hasn't been changed first
	_, statz, err := treefmt(t, "-C", tempDir)
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   32,
		stats.Formatted: 32,
		stats.Changed:   0,
	})

	// use env
	t.Setenv("TREEFMT_WORKING_DIR", tempDir)
	_, statz, err = treefmt(t, "-c")
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   32,
		stats.Formatted: 32,
		stats.Changed:   0,
	})
}

func TestFailOnChange(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)
	configPath := tempDir + "/touch.toml"

	// test without any excludes
	cfg := &config.Config{
		FormatterConfigs: map[string]*config.Formatter{
			"touch": {
				Command:  "touch",
				Includes: []string{"*"},
			},
		},
	}

	test.WriteConfig(t, configPath, cfg)
	_, _, err := treefmt(t, "--fail-on-change", "--config-file", configPath, "--tree-root", tempDir)
	as.ErrorIs(err, formatCmd.ErrFailOnChange)

	// test with no cache
	t.Setenv("TREEFMT_FAIL_ON_CHANGE", "true")
	test.WriteConfig(t, configPath, cfg)
	_, _, err = treefmt(t, "--config-file", configPath, "--tree-root", tempDir, "--no-cache")
	as.ErrorIs(err, formatCmd.ErrFailOnChange)
}

func TestBustCacheOnFormatterChange(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)
	configPath := tempDir + "/touch.toml"

	// symlink some formatters into temp dir, so we can mess with their mod times
	binPath := filepath.Join(tempDir, "bin")
	as.NoError(os.Mkdir(binPath, 0o755))

	binaries := []string{"black", "elm-format", "gofmt"}

	for _, name := range binaries {
		src, err := exec.LookPath(name)
		as.NoError(err)
		as.NoError(os.Symlink(src, filepath.Join(binPath, name)))
	}

	// prepend our test bin directory to PATH
	t.Setenv("PATH", binPath+":"+os.Getenv("PATH"))

	// start with 2 formatters
	cfg := &config.Config{
		FormatterConfigs: map[string]*config.Formatter{
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

	test.WriteConfig(t, configPath, cfg)
	args := []string{"--config-file", configPath, "--tree-root", tempDir}
	_, statz, err := treefmt(t, args...)
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   3,
		stats.Formatted: 3,
		stats.Changed:   0,
	})

	// tweak mod time of elm formatter
	newTime := time.Now().Add(-time.Minute)
	as.NoError(test.Lutimes(t, filepath.Join(binPath, "elm-format"), newTime, newTime))

	_, statz, err = treefmt(t, args...)
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   3,
		stats.Formatted: 1,
		stats.Changed:   0,
	})

	// check cache is working
	_, statz, err = treefmt(t, args...)
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   3,
		stats.Formatted: 0,
		stats.Changed:   0,
	})

	// tweak mod time of python formatter
	as.NoError(test.Lutimes(t, filepath.Join(binPath, "black"), newTime, newTime))

	_, statz, err = treefmt(t, args...)
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   3,
		stats.Formatted: 2,
		stats.Changed:   0,
	})

	// check cache is working
	_, statz, err = treefmt(t, args...)
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   3,
		stats.Formatted: 0,
		stats.Changed:   0,
	})

	// add go formatter
	goFormatter := &config.Formatter{
		Command:  "gofmt",
		Options:  []string{"-w"},
		Includes: []string{"*.go"},
	}
	cfg.FormatterConfigs["go"] = goFormatter
	test.WriteConfig(t, configPath, cfg)

	_, statz, err = treefmt(t, args...)
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   4,
		stats.Formatted: 1,
		stats.Changed:   0,
	})

	// check cache is working
	_, statz, err = treefmt(t, args...)
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   4,
		stats.Formatted: 0,
		stats.Changed:   0,
	})

	// tweak go formatter options
	goFormatter.Options = []string{"-w", "-s"}

	test.WriteConfig(t, configPath, cfg)

	_, statz, err = treefmt(t, args...)
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   4,
		stats.Formatted: 1,
		stats.Changed:   0,
	})

	// add a priority
	cfg.FormatterConfigs["go"].Priority = 3
	test.WriteConfig(t, configPath, cfg)

	_, statz, err = treefmt(t, args...)
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   4,
		stats.Formatted: 1,
		stats.Changed:   0,
	})

	// remove python formatter
	delete(cfg.FormatterConfigs, "python")
	test.WriteConfig(t, configPath, cfg)

	_, statz, err = treefmt(t, args...)
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   2,
		stats.Formatted: 0,
		stats.Changed:   0,
	})

	// check cache is working
	_, statz, err = treefmt(t, args...)
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   2,
		stats.Formatted: 0,
		stats.Changed:   0,
	})

	// remove elm formatter
	delete(cfg.FormatterConfigs, "elm")
	test.WriteConfig(t, configPath, cfg)

	_, statz, err = treefmt(t, args...)
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   1,
		stats.Formatted: 0,
		stats.Changed:   0,
	})

	// check cache is working
	_, statz, err = treefmt(t, args...)
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   1,
		stats.Formatted: 0,
		stats.Changed:   0,
	})
}

func TestGit(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)
	configPath := filepath.Join(tempDir, "/treefmt.toml")

	// basic config
	cfg := &config.Config{
		FormatterConfigs: map[string]*config.Formatter{
			"echo": {
				Command:  "echo",
				Includes: []string{"*"},
			},
		},
	}

	test.WriteConfig(t, configPath, cfg)

	run := func(traversed int, matched int, formatted int, changed int) {
		_, statz, err := treefmt(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
		as.NoError(err)

		assertStats(t, as, statz, map[stats.Type]int{
			stats.Traversed: traversed,
			stats.Matched:   matched,
			stats.Formatted: formatted,
			stats.Changed:   changed,
		})
	}

	// init a git repo
	gitCmd := exec.Command("git", "init")
	gitCmd.Dir = tempDir
	as.NoError(gitCmd.Run(), "failed to init git repository")

	// run before adding anything to the index
	run(0, 0, 0, 0)

	// add everything to the index
	gitCmd = exec.Command("git", "add", ".")
	gitCmd.Dir = tempDir
	as.NoError(gitCmd.Run(), "failed to add everything to the index")

	run(32, 32, 32, 0)

	// remove python directory from the index
	gitCmd = exec.Command("git", "rm", "--cached", "python/*")
	gitCmd.Dir = tempDir
	as.NoError(gitCmd.Run(), "failed to remove python directory from the index")

	run(29, 29, 29, 0)

	// remove nixpkgs.toml from the filesystem but leave it in the index
	as.NoError(os.Remove(filepath.Join(tempDir, "nixpkgs.toml")))
	run(28, 28, 28, 0)

	// walk with filesystem instead of with git
	// the .git folder contains 49 additional files
	// when added to the 31 we started with (32 minus nixpkgs.toml which we removed from the filesystem), we should
	// traverse 80 files.
	_, statz, err := treefmt(t, "-c", "--config-file", configPath, "--tree-root", tempDir, "--walk", "filesystem")
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 80,
		stats.Matched:   80,
		stats.Formatted: 80,
		stats.Changed:   0,
	})

	// capture current cwd, so we can replace it after the test is finished
	cwd, err := os.Getwd()
	as.NoError(err)

	t.Cleanup(func() {
		// return to the previous working directory
		as.NoError(os.Chdir(cwd))
	})

	// format specific sub paths
	_, statz, err = treefmt(t, "-C", tempDir, "-c", "go", "-vv")
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 2,
		stats.Matched:   2,
		stats.Changed:   0,
	})

	_, statz, err = treefmt(t, "-C", tempDir, "-c", "go", "haskell")
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 9,
		stats.Matched:   9,
		stats.Changed:   0,
	})

	_, statz, err = treefmt(t, "-C", tempDir, "-c", "go", "haskell", "ruby")
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 10,
		stats.Matched:   10,
		stats.Changed:   0,
	})

	// try with a bad path
	_, _, err = treefmt(t, "-C", tempDir, "-c", "haskell", "foo")
	as.ErrorContains(err, "path foo not found")

	// try with a path not in the git index, e.g. it is skipped
	_, err = os.Create(filepath.Join(tempDir, "foo.txt"))
	as.NoError(err)

	_, statz, err = treefmt(t, "-C", tempDir, "-c", "haskell", "foo.txt")
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 8,
		stats.Matched:   8,
		stats.Changed:   0,
	})

	_, statz, err = treefmt(t, "-C", tempDir, "-c", "foo.txt")
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 1,
		stats.Matched:   1,
		stats.Changed:   0,
	})
}

func TestPathsArg(t *testing.T) {
	as := require.New(t)

	// capture current cwd, so we can replace it after the test is finished
	cwd, err := os.Getwd()
	as.NoError(err)

	t.Cleanup(func() {
		// return to the previous working directory
		as.NoError(os.Chdir(cwd))
	})

	// create a project root under a temp dir, in order verify behavior with
	// files inside of temp dir, but outside of the project root
	tempDir := t.TempDir()
	treeRoot := filepath.Join(tempDir, "tree-root")
	test.TempExamplesInDir(t, treeRoot)

	configPath := filepath.Join(treeRoot, "/treefmt.toml")

	// create a file outside of treeRoot
	externalFile, err := os.Create(filepath.Join(tempDir, "outside_tree.go"))
	as.NoError(err)

	// change working directory to project root
	as.NoError(os.Chdir(treeRoot))

	// basic config
	cfg := &config.Config{
		FormatterConfigs: map[string]*config.Formatter{
			"echo": {
				Command:  "echo",
				Includes: []string{"*"},
			},
		},
	}

	test.WriteConfig(t, configPath, cfg)

	// without any path args
	_, statz, err := treefmt(t)
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 32,
		stats.Matched:   32,
		stats.Formatted: 32,
		stats.Changed:   0,
	})

	// specify some explicit paths
	_, statz, err = treefmt(t, "-c", "elm/elm.json", "haskell/Nested/Foo.hs")
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 2,
		stats.Matched:   2,
		stats.Formatted: 2,
		stats.Changed:   0,
	})

	// specify an absolute path
	absoluteInternalPath, err := filepath.Abs("elm/elm.json")
	as.NoError(err)

	_, statz, err = treefmt(t, "-c", absoluteInternalPath)
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 1,
		stats.Matched:   1,
		stats.Formatted: 1,
		stats.Changed:   0,
	})

	// specify a bad path
	_, _, err = treefmt(t, "-c", "elm/elm.json", "haskell/Nested/Bar.hs")
	as.Errorf(err, "path haskell/Nested/Bar.hs not found")

	// specify an absolute path outside the tree root
	absoluteExternalPath, err := filepath.Abs(externalFile.Name())
	as.NoError(err)
	as.FileExists(absoluteExternalPath, "exernal file must exist")
	_, _, err = treefmt(t, "-c", absoluteExternalPath)
	as.Errorf(err, "path %s not found within the tree root", absoluteExternalPath)

	// specify a relative path outside the tree root
	relativeExternalPath := "../outside_tree.go"
	as.FileExists(relativeExternalPath, "exernal file must exist")
	_, _, err = treefmt(t, "-c", relativeExternalPath)
	as.Errorf(err, "path %s not found within the tree root", relativeExternalPath)
}

func TestStdin(t *testing.T) {
	as := require.New(t)

	// capture current cwd, so we can replace it after the test is finished
	cwd, err := os.Getwd()
	as.NoError(err)
	t.Cleanup(func() {
		// return to the previous working directory
		as.NoError(os.Chdir(cwd))
	})

	tempDir := test.TempExamples(t)

	// capture current stdin and replace it on test cleanup
	prevStdIn := os.Stdin

	t.Cleanup(func() {
		os.Stdin = prevStdIn
	})

	// omit the required filename parameter
	contents := `{ foo, ... }: "hello"`
	os.Stdin = test.TempFile(t, "", "stdin", &contents)

	// we get an error about the missing filename parameter.
	out, _, err := treefmt(t, "-C", tempDir, "--allow-missing-formatter", "--stdin")
	as.EqualError(err, "exactly one path should be specified when using the --stdin flag")
	as.Equal("Error: exactly one path should be specified when using the --stdin flag\n", string(out))

	// now pass along the filename parameter
	os.Stdin = test.TempFile(t, "", "stdin", &contents)

	out, statz, err := treefmt(t, "-C", tempDir, "--allow-missing-formatter", "--stdin", "test.nix")
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 1,
		stats.Matched:   1,
		stats.Formatted: 1,
		stats.Changed:   1,
	})

	// the nix formatters should have reduced the example to the following
	as.Equal(`{ ...}: "hello"
`, string(out))

	// try a file that's outside of the project root
	os.Stdin = test.TempFile(t, "", "stdin", &contents)

	out, _, err = treefmt(t, "-C", tempDir, "--allow-missing-formatter", "--stdin", "../test.nix")
	as.Errorf(err, "path ../test.nix not inside the tree root %s", tempDir)
	as.Contains(string(out), "Error: path ../test.nix not inside the tree root")

	// try some markdown instead
	contents = `
| col1 | col2 |
| ---- | ---- |
| nice | fits |
| oh no! | it's ugly |
`
	os.Stdin = test.TempFile(t, "", "stdin", &contents)

	out, statz, err = treefmt(t, "-C", tempDir, "--allow-missing-formatter", "--stdin", "test.md")
	as.NoError(err)

	assertStats(t, as, statz, map[stats.Type]int{
		stats.Traversed: 1,
		stats.Matched:   1,
		stats.Formatted: 1,
		stats.Changed:   1,
	})

	as.Equal(`| col1   | col2      |
| ------ | --------- |
| nice   | fits      |
| oh no! | it's ugly |
`, string(out))
}

func TestDeterministicOrderingInPipeline(t *testing.T) {
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

	test.WriteConfig(t, configPath, &config.Config{
		FormatterConfigs: map[string]*config.Formatter{
			// a and b have no priority set, which means they default to 0 and should execute first
			// a and b should execute in lexicographical order
			// c should execute first since it has a priority of 1
			"fmt-a": {
				Command:  "test-fmt",
				Options:  []string{"fmt-a"},
				Includes: []string{"*.py"},
			},
			"fmt-b": {
				Command:  "test-fmt",
				Options:  []string{"fmt-b"},
				Includes: []string{"*.py"},
			},
			"fmt-c": {
				Command:  "test-fmt",
				Options:  []string{"fmt-c"},
				Includes: []string{"*.py"},
				Priority: 1,
			},
		},
	})

	_, _, err = treefmt(t, "-C", tempDir)
	as.NoError(err)

	matcher := regexp.MustCompile("^fmt-(.*)")

	// check each affected file for the sequence of test statements which should be prepended to the end
	sequence := []string{"fmt-a", "fmt-b", "fmt-c"}
	paths := []string{"python/main.py", "python/virtualenv_proxy.py"}

	for _, p := range paths {
		file, err := os.Open(filepath.Join(tempDir, p))
		as.NoError(err)

		scanner := bufio.NewScanner(file)
		idx := 0

		for scanner.Scan() {
			line := scanner.Text()

			matches := matcher.FindAllString(line, -1)
			if len(matches) != 1 {
				continue
			}

			as.Equal(sequence[idx], matches[0])

			idx++
		}
	}
}

func TestRunInSubdir(t *testing.T) {
	as := require.New(t)

	// Run the same test for each walk type
	for _, walkType := range walk.TypeValues() {
		t.Run(walkType.String(), func(t *testing.T) {
			// capture current cwd, so we can replace it after the test is finished
			cwd, err := os.Getwd()
			as.NoError(err)

			t.Cleanup(func() {
				// return to the previous working directory
				as.NoError(os.Chdir(cwd))
			})

			tempDir := test.TempExamples(t)
			configPath := filepath.Join(tempDir, "/treefmt.toml")

			// set the walk type via environment variable
			t.Setenv("TREEFMT_WALK_TYPE", walkType.String())

			// if we are testing git walking, init a git repo before continuing
			if walkType == walk.Git {
				// init a git repo
				gitCmd := exec.Command("git", "init")
				gitCmd.Dir = tempDir
				as.NoError(gitCmd.Run(), "failed to init git repository")

				// add everything to the index
				gitCmd = exec.Command("git", "add", ".")
				gitCmd.Dir = tempDir
				as.NoError(gitCmd.Run(), "failed to add everything to the index")
			}

			// test that formatters are resolved relative to the treefmt root
			echoPath, err := exec.LookPath("echo")
			as.NoError(err)

			echoRel := path.Join(tempDir, "echo")

			err = os.Symlink(echoPath, echoRel)
			as.NoError(err)

			// change working directory to subdirectory
			as.NoError(os.Chdir(filepath.Join(tempDir, "elm")))

			// basic config
			cfg := &config.Config{
				FormatterConfigs: map[string]*config.Formatter{
					"echo": {
						Command:  "./echo",
						Includes: []string{"*"},
					},
				},
			}
			test.WriteConfig(t, configPath, cfg)

			// without any path args, should reformat the whole tree
			_, statz, err := treefmt(t)
			as.NoError(err)

			assertStats(t, as, statz, map[stats.Type]int{
				stats.Traversed: 32,
				stats.Matched:   32,
				stats.Formatted: 32,
				stats.Changed:   0,
			})

			// specify some explicit paths, relative to the tree root
			// this should not work, as we're in a subdirectory
			_, _, err = treefmt(t, "-c", "elm/elm.json", "haskell/Nested/Foo.hs")
			as.ErrorContains(err, "path elm/elm.json not found")

			// specify some explicit paths, relative to the current directory
			_, statz, err = treefmt(t, "-c", "elm.json", "../haskell/Nested/Foo.hs")
			as.NoError(err)

			assertStats(t, as, statz, map[stats.Type]int{
				stats.Traversed: 2,
				stats.Matched:   2,
				stats.Formatted: 2,
				stats.Changed:   0,
			})
		})
	}
}

func treefmt(t *testing.T, args ...string) ([]byte, *stats.Stats, error) {
	t.Helper()

	t.Logf("treefmt %s", strings.Join(args, " "))

	tempDir := t.TempDir()
	tempOut := test.TempFile(t, tempDir, "combined_output", nil)

	// capture standard outputs before swapping them
	stdout := os.Stdout
	stderr := os.Stderr

	// swap them temporarily
	os.Stdout = tempOut
	os.Stderr = tempOut

	log.SetOutput(tempOut)

	defer func() {
		// swap outputs back
		os.Stdout = stdout
		os.Stderr = stderr
		log.SetOutput(stderr)
	}()

	// run the command
	root, statz := cmd.NewRoot()

	if args == nil {
		// we must pass an empty array otherwise cobra with use os.Args[1:]
		args = []string{}
	}

	root.SetArgs(args)
	root.SetOut(tempOut)
	root.SetErr(tempOut)

	// record the start time
	start := time.Now()

	defer func() {
		// Wait until we tick over into the next second before continuing.
		// This ensures we correctly detect changes within tests as treefmt compares modtime at second level precision.
		waitUntil := start.Truncate(time.Second).Add(time.Second)
		time.Sleep(time.Until(waitUntil))
	}()

	// execute the command
	cmdErr := root.Execute()

	// reset and read the temporary output
	if _, resetErr := tempOut.Seek(0, 0); resetErr != nil {
		t.Fatal(fmt.Errorf("failed to reset temp output for reading: %w", resetErr))
	}

	out, readErr := io.ReadAll(tempOut)
	if readErr != nil {
		t.Fatal(fmt.Errorf("failed to read temp output: %w", readErr))
	}

	t.Log(string(out))

	return out, statz, cmdErr
}

func assertStats(
	t *testing.T,
	as *require.Assertions,
	statz *stats.Stats,
	expected map[stats.Type]int,
) {
	t.Helper()

	for k, v := range expected {
		as.Equal(v, statz.Value(k), k.String())
	}
}
