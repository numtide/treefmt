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
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/numtide/treefmt/cmd"
	format2 "github.com/numtide/treefmt/cmd/format"
	"github.com/numtide/treefmt/format"
	"github.com/numtide/treefmt/stats"
	"github.com/numtide/treefmt/test"
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

	_, _, err = execute(t, "-C", tempDir, "--allow-missing-formatter", "--on-unmatched", "fatal")
	as.ErrorContains(err, fmt.Sprintf("no formatter for path: %s", paths[0]))

	checkOutput := func(level string, output []byte) {
		for _, p := range paths {
			as.Contains(string(output), fmt.Sprintf("%s format: no formatter for path: %s", level, p))
		}
	}

	var out []byte

	// default is warn
	out, _, err = execute(t, "-C", tempDir, "--allow-missing-formatter", "-c")
	as.NoError(err)
	checkOutput("WARN", out)

	out, _, err = execute(t, "-C", tempDir, "--allow-missing-formatter", "-c", "--on-unmatched", "warn")
	as.NoError(err)
	checkOutput("WARN", out)

	out, _, err = execute(t, "-C", tempDir, "--allow-missing-formatter", "-c", "-u", "error")
	as.NoError(err)
	checkOutput("ERRO", out)

	out, _, err = execute(t, "-C", tempDir, "--allow-missing-formatter", "-c", "-v", "--on-unmatched", "info")
	as.NoError(err)
	checkOutput("INFO", out)

	t.Setenv("TREEFMT_ON_UNMATCHED", "debug")
	out, _, err = execute(t, "-C", tempDir, "--allow-missing-formatter", "-c", "-vv")
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

	_, _, err = execute(t, "-C", tempDir, "--allow-missing-formatter", "--cpu-profile", "cpu.pprof")
	as.NoError(err)
	as.FileExists(filepath.Join(tempDir, "cpu.pprof"))
	_, err = os.Stat(filepath.Join(tempDir, "cpu.pprof"))
	as.NoError(err)

	t.Setenv("TREEFMT_CPU_PROFILE", "env.pprof")
	_, _, err = execute(t, "-C", tempDir, "--allow-missing-formatter")
	as.NoError(err)
	as.FileExists(filepath.Join(tempDir, "env.pprof"))
	_, err = os.Stat(filepath.Join(tempDir, "env.pprof"))
	as.NoError(err)
}

func TestAllowMissingFormatter(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)
	configPath := tempDir + "/treefmt.toml"

	cfg := map[string]any{
		"formatter": map[string]any{
			"foo-fmt": map[string]any{
				"command": "foo-fmt",
			},
		},
	}

	test.WriteConfig(t, configPath, cfg)

	_, _, err := execute(t, "--config-file", configPath, "--tree-root", tempDir)
	as.ErrorIs(err, format.ErrCommandNotFound)

	_, _, err = execute(t, "--config-file", configPath, "--tree-root", tempDir, "--allow-missing-formatter")
	as.NoError(err)

	t.Setenv("TREEFMT_ALLOW_MISSING_FORMATTER", "true")
	_, _, err = execute(t, "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
}

func TestSpecifyingFormatters(t *testing.T) {
	as := require.New(t)

	cfg := map[string]any{
		"formatter": map[string]any{
			"elm": map[string]any{
				"command":  "touch",
				"options":  []string{"-m"},
				"includes": []string{"*.elm"},
			},
			"nix": map[string]any{
				"command":  "touch",
				"options":  []string{"-m"},
				"includes": []string{"*.nix"},
			},
			"ruby": map[string]any{
				"command":  "touch",
				"options":  []string{"-m"},
				"includes": []string{"*.rb"},
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

	_, statz, err := execute(t, "-c", "--config-file", configPath, "--tree-root", tempDir)

	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 3, 3)

	setup()

	_, statz, err = execute(t, "-c", "--config-file", configPath, "--tree-root", tempDir, "--formatters", "elm,nix")

	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 2, 2)

	setup()

	_, statz, err = execute(t, "-c", "--config-file", configPath, "--tree-root", tempDir, "-f", "ruby,nix")

	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 2, 2)

	setup()

	_, statz, err = execute(t, "-c", "--config-file", configPath, "--tree-root", tempDir, "--formatters", "nix")

	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 1, 1)

	// test bad names
	setup()

	_, _, err = execute(t, "-c", "--config-file", configPath, "--tree-root", tempDir, "--formatters", "foo")

	as.Errorf(err, "formatter not found in config: foo")

	t.Setenv("TREEFMT_FORMATTERS", "bar,foo")

	_, _, err = execute(t, "-c", "--config-file", configPath, "--tree-root", tempDir)

	as.Errorf(err, "formatter not found in config: bar")
}

func TestIncludesAndExcludes(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)
	configPath := tempDir + "/touch.toml"

	// test without any excludes
	cfg := map[string]any{
		"formatter": map[string]any{
			"echo": map[string]any{
				"command":  "echo",
				"includes": []string{"*"},
			},
		},
	}

	test.WriteConfig(t, configPath, cfg)
	_, statz, err := execute(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 32, 0)

	// globally exclude nix files
	cfg["excludes"] = []string{"*.nix"}

	test.WriteConfig(t, configPath, cfg)
	_, statz, err = execute(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 31, 0)

	// add haskell files to the global exclude
	cfg["excludes"] = []string{"*.nix", "*.hs"}

	test.WriteConfig(t, configPath, cfg)
	_, statz, err = execute(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 25, 0)

	echo := cfg["formatter"].(map[string]any)["echo"].(map[string]any)

	// remove python files from the echo formatter
	echo["excludes"] = []string{"*.py"}

	test.WriteConfig(t, configPath, cfg)
	_, statz, err = execute(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 23, 0)

	// remove go files from the echo formatter via env
	t.Setenv("TREEFMT_FORMATTER_ECHO_EXCLUDES", "*.py,*.go")

	test.WriteConfig(t, configPath, cfg)
	_, statz, err = execute(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 22, 0)

	t.Setenv("TREEFMT_FORMATTER_ECHO_EXCLUDES", "") // reset

	// adjust the includes for echo to only include elm files
	echo["includes"] = []string{"*.elm"}

	test.WriteConfig(t, configPath, cfg)
	_, statz, err = execute(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 1, 0)

	// add js files to echo formatter via env
	t.Setenv("TREEFMT_FORMATTER_ECHO_INCLUDES", "*.elm,*.js")

	test.WriteConfig(t, configPath, cfg)
	_, statz, err = execute(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 2, 0)
}

func TestPrjRootEnvVariable(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)
	configPath := tempDir + "/treefmt.toml"

	// test without any excludes
	cfg := map[string]any{
		"formatter": map[string]any{
			"echo": map[string]any{
				"command":  "echo",
				"includes": []string{"*"},
			},
		},
	}

	test.WriteConfig(t, configPath, cfg)
	t.Setenv("PRJ_ROOT", tempDir)
	_, statz, err := execute(t, "--config-file", configPath)
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 32, 0)
}

func TestCache(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)
	configPath := tempDir + "/touch.toml"

	// test without any excludes
	cfg := map[string]any{
		"formatter": map[string]any{
			"echo": map[string]any{
				"command":  "echo",
				"includes": []string{"*"},
			},
		},
	}

	var err error

	test.WriteConfig(t, configPath, cfg)
	_, statz, err := execute(t, "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 32, 0)

	_, statz, err = execute(t, "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	assertStats(t, as, statz, 32, 0, 0, 0)

	// clear cache
	_, statz, err = execute(t, "--config-file", configPath, "--tree-root", tempDir, "-c")
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 32, 0)

	_, statz, err = execute(t, "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	assertStats(t, as, statz, 32, 0, 0, 0)

	// clear cache
	_, statz, err = execute(t, "--config-file", configPath, "--tree-root", tempDir, "-c")
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 32, 0)

	_, statz, err = execute(t, "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	assertStats(t, as, statz, 32, 0, 0, 0)

	// no cache
	_, statz, err = execute(t, "--config-file", configPath, "--tree-root", tempDir, "--no-cache")
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 32, 0)
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
	cfg := map[string]any{
		"formatter": map[string]any{
			"echo": map[string]any{
				"command":  "echo",
				"includes": []string{"*"},
			},
		},
	}

	test.WriteConfig(t, configPath, cfg)

	// by default, we look for ./treefmt.toml and use the cwd for the tree root
	// this should fail if the working directory hasn't been changed first
	_, statz, err := execute(t, "-C", tempDir)
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 32, 0)

	// use env
	t.Setenv("TREEFMT_WORKING_DIR", tempDir)
	_, statz, err = execute(t, "-c")
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 32, 0)
}

func TestFailOnChange(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)
	configPath := tempDir + "/touch.toml"

	// test without any excludes
	cfg := map[string]any{
		"formatter": map[string]any{
			"touch": map[string]any{
				"command":  "touch",
				"includes": []string{"*"},
			},
		},
	}

	test.WriteConfig(t, configPath, cfg)
	_, _, err := execute(t, "--fail-on-change", "--config-file", configPath, "--tree-root", tempDir)
	as.ErrorIs(err, format2.ErrFailOnChange)

	// we have second precision mod time tracking
	time.Sleep(time.Second)

	// test with no cache
	t.Setenv("TREEFMT_FAIL_ON_CHANGE", "true")
	test.WriteConfig(t, configPath, cfg)
	_, _, err = execute(t, "--config-file", configPath, "--tree-root", tempDir, "--no-cache")
	as.ErrorIs(err, format2.ErrFailOnChange)
}

func TestBustCacheOnFormatterChange(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)
	configPath := tempDir + "/touch.toml"

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
	t.Setenv("PATH", binPath+":"+os.Getenv("PATH"))

	// start with 2 formatters
	cfg := map[string]any{
		"formatter": map[string]any{
			"python": map[string]any{
				"command":  "black",
				"includes": []string{"*.py"},
			},
			"elm": map[string]any{
				"command":  "elm-format",
				"options":  []string{"--yes"},
				"includes": []string{"*.elm"},
			},
		},
	}

	test.WriteConfig(t, configPath, cfg)
	args := []string{"--config-file", configPath, "--tree-root", tempDir}
	_, statz, err := execute(t, args...)
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 3, 0)

	// tweak mod time of elm formatter
	as.NoError(test.RecreateSymlink(t, binPath+"/"+"elm-format"))

	_, statz, err = execute(t, args...)
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 3, 0)

	// check cache is working
	_, statz, err = execute(t, args...)
	as.NoError(err)
	assertStats(t, as, statz, 32, 0, 0, 0)

	// tweak mod time of python formatter
	as.NoError(test.RecreateSymlink(t, binPath+"/"+"black"))

	_, statz, err = execute(t, args...)
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 3, 0)

	// check cache is working
	_, statz, err = execute(t, args...)
	as.NoError(err)
	assertStats(t, as, statz, 32, 0, 0, 0)

	// add go formatter
	formatters := cfg["formatter"].(map[string]any)

	formatters["go"] = map[string]any{
		"command":  "gofmt",
		"options":  []string{"-w"},
		"includes": []string{"*.go"},
	}

	test.WriteConfig(t, configPath, cfg)

	_, statz, err = execute(t, args...)
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 4, 0)

	// check cache is working
	_, statz, err = execute(t, args...)
	as.NoError(err)
	assertStats(t, as, statz, 32, 0, 0, 0)

	// remove python formatter
	delete(formatters, "python")
	test.WriteConfig(t, configPath, cfg)

	_, statz, err = execute(t, args...)
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 2, 0)

	// check cache is working
	_, statz, err = execute(t, args...)
	as.NoError(err)
	assertStats(t, as, statz, 32, 0, 0, 0)

	// remove elm formatter
	delete(formatters, "elm")
	test.WriteConfig(t, configPath, cfg)

	_, statz, err = execute(t, args...)
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 1, 0)

	// check cache is working
	_, statz, err = execute(t, args...)
	as.NoError(err)
	assertStats(t, as, statz, 32, 0, 0, 0)
}

func TestGitWorktree(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)
	configPath := filepath.Join(tempDir, "/treefmt.toml")

	// basic config
	cfg := map[string]any{
		"formatter": map[string]any{
			"echo": map[string]any{
				"command":  "echo",
				"includes": []string{"*"},
			},
		},
	}

	test.WriteConfig(t, configPath, cfg)

	// init a git repo
	repo, err := git.Init(
		filesystem.NewStorage(
			osfs.New(path.Join(tempDir, ".git")),
			cache.NewObjectLRUDefault(),
		),
		osfs.New(tempDir),
	)
	as.NoError(err, "failed to init git repository")

	// get worktree
	wt, err := repo.Worktree()
	as.NoError(err, "failed to get git worktree")

	run := func(traversed int32, emitted int32, matched int32, formatted int32) {
		_, statz, err := execute(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
		as.NoError(err)
		assertStats(t, as, statz, traversed, emitted, matched, formatted)
	}

	// run before adding anything to the worktree
	run(0, 0, 0, 0)

	// add everything to the worktree
	as.NoError(wt.AddGlob("."))
	as.NoError(err)
	run(32, 32, 32, 0)

	// remove python directory from the worktree
	as.NoError(wt.RemoveGlob("python/*"))
	run(29, 29, 29, 0)

	// remove nixpkgs.toml from the filesystem but leave it in the index
	as.NoError(os.Remove(filepath.Join(tempDir, "nixpkgs.toml")))
	run(28, 28, 28, 0)

	// walk with filesystem instead of git
	_, statz, err := execute(t, "-c", "--config-file", configPath, "--tree-root", tempDir, "--walk", "filesystem")
	as.NoError(err)
	assertStats(t, as, statz, 60, 60, 60, 0)

	// capture current cwd, so we can replace it after the test is finished
	cwd, err := os.Getwd()
	as.NoError(err)

	t.Cleanup(func() {
		// return to the previous working directory
		as.NoError(os.Chdir(cwd))
	})

	// format specific sub paths
	_, statz, err = execute(t, "-C", tempDir, "-c", "go", "-vv")
	as.NoError(err)
	assertStats(t, as, statz, 2, 2, 2, 0)

	_, statz, err = execute(t, "-C", tempDir, "-c", "go", "haskell")
	as.NoError(err)
	assertStats(t, as, statz, 9, 9, 9, 0)

	_, statz, err = execute(t, "-C", tempDir, "-c", "go", "haskell", "ruby")
	as.NoError(err)
	assertStats(t, as, statz, 10, 10, 10, 0)

	// try with a bad path
	_, _, err = execute(t, "-C", tempDir, "-c", "haskell", "foo")
	as.ErrorContains(err, "path foo not found within the tree root")

	// try with a path not in the git index, e.g. it is skipped
	_, err = os.Create(filepath.Join(tempDir, "foo.txt"))
	as.NoError(err)

	_, statz, err = execute(t, "-C", tempDir, "-c", "haskell", "foo.txt")
	as.NoError(err)
	assertStats(t, as, statz, 7, 7, 7, 0)

	_, statz, err = execute(t, "-C", tempDir, "-c", "foo.txt")
	as.NoError(err)
	assertStats(t, as, statz, 0, 0, 0, 0)
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

	tempDir := test.TempExamples(t)
	configPath := filepath.Join(tempDir, "/treefmt.toml")

	// change working directory to temp root
	as.NoError(os.Chdir(tempDir))

	// basic config
	cfg := map[string]any{
		"formatter": map[string]any{
			"echo": map[string]any{
				"command":  "echo",
				"includes": []string{"*"},
			},
		},
	}
	test.WriteConfig(t, configPath, cfg)

	// without any path args
	_, statz, err := execute(t)
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 32, 0)

	// specify some explicit paths
	_, statz, err = execute(t, "-c", "elm/elm.json", "haskell/Nested/Foo.hs")
	as.NoError(err)
	assertStats(t, as, statz, 2, 2, 2, 0)

	// specify a bad path
	_, _, err = execute(t, "-c", "elm/elm.json", "haskell/Nested/Bar.hs")
	as.ErrorContains(err, "path haskell/Nested/Bar.hs not found within the tree root")

	// specify a path outside the tree root
	externalPath := filepath.Join(cwd, "go.mod")
	_, _, err = execute(t, "-c", externalPath)
	as.ErrorContains(err, fmt.Sprintf("path %s not found within the tree root", externalPath))
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
	out, _, err := execute(t, "-C", tempDir, "--allow-missing-formatter", "--stdin")
	as.EqualError(err, "exactly one path should be specified when using the --stdin flag")
	as.Equal("", string(out))

	// now pass along the filename parameter
	contents = `{ foo, ... }: "hello"`
	os.Stdin = test.TempFile(t, "", "stdin", &contents)

	t.Setenv("TREEFMT_STDIN", "true")
	out, statz, err := execute(t, "-C", tempDir, "--allow-missing-formatter", "test.nix")
	as.NoError(err)
	assertStats(t, as, statz, 1, 1, 1, 1)

	// the nix formatters should have reduced the example to the following
	as.Equal(`{ ...}: "hello"
`, string(out))

	// try some markdown instead
	contents = `
| col1 | col2 |
| ---- | ---- |
| nice | fits |
| oh no! | it's ugly |
`
	os.Stdin = test.TempFile(t, "", "stdin", &contents)

	out, statz, err = execute(t, "-C", tempDir, "--allow-missing-formatter", "--stdin", "test.md")
	as.NoError(err)
	assertStats(t, as, statz, 1, 1, 1, 1)

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

	cfg := map[string]any{
		"formatter": map[string]any{
			"fmt-a": map[string]any{
				"command":  "test-fmt",
				"options":  []string{"fmt-a"},
				"includes": []string{"*.py"},
			},
			"fmt-b": map[string]any{
				"command":  "test-fmt",
				"options":  []string{"fmt-b"},
				"includes": []string{"*.py"},
			},
			"fmt-c": map[string]any{
				"command":  "test-fmt",
				"options":  []string{"fmt-c"},
				"includes": []string{"*.py"},
				"priority": 1,
			},
		},
	}
	test.WriteConfig(t, configPath, cfg)

	_, _, err = execute(t, "-C", tempDir)
	as.NoError(err)

	matcher := regexp.MustCompile("^fmt-(.*)")

	// check each affected file for the sequence of test statements which should be prepended to the end
	sequence := []string{"fmt-a", "fmt-b", "fmt-c"}
	paths := []string{"python/main.py", "python/virtualenv_proxy.py"}

	for _, p := range paths {
		file, err := os.Open(filepath.Join(tempDir, p))
		as.NoError(err)

		idx := 0
		scanner := bufio.NewScanner(file)

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

	// capture current cwd, so we can replace it after the test is finished
	cwd, err := os.Getwd()
	as.NoError(err)

	t.Cleanup(func() {
		// return to the previous working directory
		as.NoError(os.Chdir(cwd))
	})

	tempDir := test.TempExamples(t)
	configPath := filepath.Join(tempDir, "/treefmt.toml")

	// Also test that formatters are resolved relative to the treefmt root
	echoPath, err := exec.LookPath("echo")
	as.NoError(err)

	echoRel := path.Join(tempDir, "echo")
	err = os.Symlink(echoPath, echoRel)

	as.NoError(err)

	// change working directory to sub directory
	as.NoError(os.Chdir(filepath.Join(tempDir, "elm")))

	// basic config
	cfg := map[string]any{
		"formatter": map[string]any{
			"echo": map[string]any{
				"command":  "./echo",
				"includes": []string{"*"},
			},
		},
	}
	test.WriteConfig(t, configPath, cfg)

	// without any path args, should reformat the whole tree
	_, statz, err := execute(t)
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 32, 0)

	// specify some explicit paths, relative to the tree root
	_, statz, err = execute(t, "-c", "elm/elm.json", "haskell/Nested/Foo.hs")
	as.NoError(err)
	assertStats(t, as, statz, 2, 2, 2, 0)
}

func execute(t *testing.T, args ...string) ([]byte, *stats.Stats, error) {
	t.Helper()

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

	if err := root.Execute(); err != nil {
		return nil, nil, err
	}

	// reset and read the temporary output
	if _, err := tempOut.Seek(0, 0); err != nil {
		return nil, nil, fmt.Errorf("failed to reset temp output for reading: %w", err)
	}

	out, err := io.ReadAll(tempOut)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read temp output: %w", err)
	}

	return out, statz, nil
}

func assertStats(
	t *testing.T,
	as *require.Assertions,
	statz *stats.Stats,
	traversed int32,
	emitted int32,
	matched int32,
	formatted int32,
) {
	t.Helper()
	as.Equal(traversed, statz.Value(stats.Traversed), "stats.traversed")
	as.Equal(emitted, statz.Value(stats.Emitted), "stats.emitted")
	as.Equal(matched, statz.Value(stats.Matched), "stats.matched")
	as.Equal(formatted, statz.Value(stats.Formatted), "stats.formatted")
}
