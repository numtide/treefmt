package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"git.numtide.com/numtide/treefmt/config"
	"git.numtide.com/numtide/treefmt/format"
	"git.numtide.com/numtide/treefmt/test"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/storage/filesystem"

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

	_, err = cmd(t, "-C", tempDir, "--allow-missing-formatter", "--on-unmatched", "fatal")
	as.ErrorContains(err, fmt.Sprintf("no formatter for path: %s", paths[0]))

	checkOutput := func(level string, output []byte) {
		for _, p := range paths {
			as.Contains(string(output), fmt.Sprintf("%s format: no formatter for path: %s", level, p))
		}
	}

	var out []byte

	// default is warn
	out, err = cmd(t, "-C", tempDir, "--allow-missing-formatter", "-c")
	as.NoError(err)
	checkOutput("WARN", out)

	out, err = cmd(t, "-C", tempDir, "--allow-missing-formatter", "-c", "--on-unmatched", "warn")
	as.NoError(err)
	checkOutput("WARN", out)

	out, err = cmd(t, "-C", tempDir, "--allow-missing-formatter", "-c", "-u", "error")
	as.NoError(err)
	checkOutput("ERRO", out)

	out, err = cmd(t, "-C", tempDir, "--allow-missing-formatter", "-c", "-v", "--on-unmatched", "info")
	as.NoError(err)
	checkOutput("INFO", out)

	out, err = cmd(t, "-C", tempDir, "--allow-missing-formatter", "-c", "-vv", "-u", "debug")
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

	_, err = cmd(t, "-C", tempDir, "--allow-missing-formatter", "--cpu-profile", "cpu.pprof")
	as.NoError(err)
	as.FileExists(filepath.Join(tempDir, "cpu.pprof"))
	_, err = os.Stat(filepath.Join(tempDir, "cpu.pprof"))
	as.NoError(err)
}

func TestAllowMissingFormatter(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)
	configPath := tempDir + "/treefmt.toml"

	test.WriteConfig(t, configPath, config.Config{
		Formatters: map[string]*config.Formatter{
			"foo-fmt": {
				Command: "foo-fmt",
			},
		},
	})

	_, err := cmd(t, "--config-file", configPath, "--tree-root", tempDir)
	as.ErrorIs(err, format.ErrCommandNotFound)

	_, err = cmd(t, "--config-file", configPath, "--tree-root", tempDir, "--allow-missing-formatter")
	as.NoError(err)
}

func TestSpecifyingFormatters(t *testing.T) {
	as := require.New(t)

	cfg := config.Config{
		Formatters: map[string]*config.Formatter{
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
	_, err := cmd(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	assertStats(t, as, 32, 32, 3, 3)

	setup()
	_, err = cmd(t, "-c", "--config-file", configPath, "--tree-root", tempDir, "--formatters", "elm,nix")
	as.NoError(err)
	assertStats(t, as, 32, 32, 2, 2)

	setup()
	_, err = cmd(t, "-c", "--config-file", configPath, "--tree-root", tempDir, "-f", "ruby,nix")
	as.NoError(err)
	assertStats(t, as, 32, 32, 2, 2)

	setup()
	_, err = cmd(t, "-c", "--config-file", configPath, "--tree-root", tempDir, "--formatters", "nix")
	as.NoError(err)
	assertStats(t, as, 32, 32, 1, 1)

	// test bad names
	setup()
	_, err = cmd(t, "-c", "--config-file", configPath, "--tree-root", tempDir, "--formatters", "foo")
	as.Errorf(err, "formatter not found in config: foo")

	_, err = cmd(t, "-c", "--config-file", configPath, "--tree-root", tempDir, "--formatters", "bar,foo")
	as.Errorf(err, "formatter not found in config: bar")
}

func TestIncludesAndExcludes(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)
	configPath := tempDir + "/touch.toml"

	// test without any excludes
	cfg := config.Config{
		Formatters: map[string]*config.Formatter{
			"echo": {
				Command:  "echo",
				Includes: []string{"*"},
			},
		},
	}

	test.WriteConfig(t, configPath, cfg)
	_, err := cmd(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	assertStats(t, as, 32, 32, 32, 0)

	// globally exclude nix files
	cfg.Global.Excludes = []string{"*.nix"}

	test.WriteConfig(t, configPath, cfg)
	_, err = cmd(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	assertStats(t, as, 32, 32, 31, 0)

	// add haskell files to the global exclude
	cfg.Global.Excludes = []string{"*.nix", "*.hs"}

	test.WriteConfig(t, configPath, cfg)
	_, err = cmd(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	assertStats(t, as, 32, 32, 25, 0)

	echo := cfg.Formatters["echo"]

	// remove python files from the echo formatter
	echo.Excludes = []string{"*.py"}

	test.WriteConfig(t, configPath, cfg)
	_, err = cmd(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	assertStats(t, as, 32, 32, 23, 0)

	// remove go files from the echo formatter
	echo.Excludes = []string{"*.py", "*.go"}

	test.WriteConfig(t, configPath, cfg)
	_, err = cmd(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	assertStats(t, as, 32, 32, 22, 0)

	// adjust the includes for echo to only include elm files
	echo.Includes = []string{"*.elm"}

	test.WriteConfig(t, configPath, cfg)
	_, err = cmd(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	assertStats(t, as, 32, 32, 1, 0)

	// add js files to echo formatter
	echo.Includes = []string{"*.elm", "*.js"}

	test.WriteConfig(t, configPath, cfg)
	_, err = cmd(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	assertStats(t, as, 32, 32, 2, 0)
}

func TestCache(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)
	configPath := tempDir + "/touch.toml"

	// test without any excludes
	cfg := config.Config{
		Formatters: map[string]*config.Formatter{
			"echo": {
				Command:  "echo",
				Includes: []string{"*"},
			},
		},
	}

	var err error

	test.WriteConfig(t, configPath, cfg)
	_, err = cmd(t, "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	assertStats(t, as, 32, 32, 32, 0)

	_, err = cmd(t, "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	assertStats(t, as, 32, 0, 0, 0)

	// clear cache
	_, err = cmd(t, "--config-file", configPath, "--tree-root", tempDir, "-c")
	as.NoError(err)
	assertStats(t, as, 32, 32, 32, 0)

	_, err = cmd(t, "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	assertStats(t, as, 32, 0, 0, 0)

	// clear cache
	_, err = cmd(t, "--config-file", configPath, "--tree-root", tempDir, "-c")
	as.NoError(err)
	assertStats(t, as, 32, 32, 32, 0)

	_, err = cmd(t, "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	assertStats(t, as, 32, 0, 0, 0)

	// no cache
	_, err = cmd(t, "--config-file", configPath, "--tree-root", tempDir, "--no-cache")
	as.NoError(err)
	assertStats(t, as, 32, 32, 32, 0)
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
	cfg := config.Config{
		Formatters: map[string]*config.Formatter{
			"echo": {
				Command:  "echo",
				Includes: []string{"*"},
			},
		},
	}

	test.WriteConfig(t, configPath, cfg)

	// by default, we look for ./treefmt.toml and use the cwd for the tree root
	// this should fail if the working directory hasn't been changed first
	_, err = cmd(t, "-C", tempDir)
	as.NoError(err)
	assertStats(t, as, 32, 32, 32, 0)
}

func TestFailOnChange(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)
	configPath := tempDir + "/touch.toml"

	// test without any excludes
	cfg := config.Config{
		Formatters: map[string]*config.Formatter{
			"touch": {
				Command:  "touch",
				Includes: []string{"*"},
			},
		},
	}

	test.WriteConfig(t, configPath, cfg)
	_, err := cmd(t, "--fail-on-change", "--config-file", configPath, "--tree-root", tempDir)
	as.ErrorIs(err, ErrFailOnChange)

	// we have second precision mod time tracking
	time.Sleep(time.Second)

	// test with no cache
	test.WriteConfig(t, configPath, cfg)
	_, err = cmd(t, "--fail-on-change", "--config-file", configPath, "--tree-root", tempDir, "--no-cache")
	as.ErrorIs(err, ErrFailOnChange)
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
	as.NoError(os.Setenv("PATH", binPath+":"+os.Getenv("PATH")))

	// start with 2 formatters
	cfg := config.Config{
		Formatters: map[string]*config.Formatter{
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
	_, err := cmd(t, args...)
	as.NoError(err)
	assertStats(t, as, 32, 32, 3, 0)

	// tweak mod time of elm formatter
	as.NoError(test.RecreateSymlink(t, binPath+"/"+"elm-format"))

	_, err = cmd(t, args...)
	as.NoError(err)
	assertStats(t, as, 32, 32, 3, 0)

	// check cache is working
	_, err = cmd(t, args...)
	as.NoError(err)
	assertStats(t, as, 32, 0, 0, 0)

	// tweak mod time of python formatter
	as.NoError(test.RecreateSymlink(t, binPath+"/"+"black"))

	_, err = cmd(t, args...)
	as.NoError(err)
	assertStats(t, as, 32, 32, 3, 0)

	// check cache is working
	_, err = cmd(t, args...)
	as.NoError(err)
	assertStats(t, as, 32, 0, 0, 0)

	// add go formatter
	cfg.Formatters["go"] = &config.Formatter{
		Command:  "gofmt",
		Options:  []string{"-w"},
		Includes: []string{"*.go"},
	}
	test.WriteConfig(t, configPath, cfg)

	_, err = cmd(t, args...)
	as.NoError(err)
	assertStats(t, as, 32, 32, 4, 0)

	// check cache is working
	_, err = cmd(t, args...)
	as.NoError(err)
	assertStats(t, as, 32, 0, 0, 0)

	// remove python formatter
	delete(cfg.Formatters, "python")
	test.WriteConfig(t, configPath, cfg)

	_, err = cmd(t, args...)
	as.NoError(err)
	assertStats(t, as, 32, 32, 2, 0)

	// check cache is working
	_, err = cmd(t, args...)
	as.NoError(err)
	assertStats(t, as, 32, 0, 0, 0)

	// remove elm formatter
	delete(cfg.Formatters, "elm")
	test.WriteConfig(t, configPath, cfg)

	_, err = cmd(t, args...)
	as.NoError(err)
	assertStats(t, as, 32, 32, 1, 0)

	// check cache is working
	_, err = cmd(t, args...)
	as.NoError(err)
	assertStats(t, as, 32, 0, 0, 0)
}

func TestGitWorktree(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)
	configPath := filepath.Join(tempDir, "/treefmt.toml")

	// basic config
	cfg := config.Config{
		Formatters: map[string]*config.Formatter{
			"echo": {
				Command:  "echo",
				Includes: []string{"*"},
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
		_, err := cmd(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
		as.NoError(err)
		assertStats(t, as, traversed, emitted, matched, formatted)
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
	_, err = cmd(t, "-c", "--config-file", configPath, "--tree-root", tempDir, "--walk", "filesystem")
	as.NoError(err)
	assertStats(t, as, 60, 60, 60, 0)
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
	cfg := config.Config{
		Formatters: map[string]*config.Formatter{
			"echo": {
				Command:  "echo",
				Includes: []string{"*"},
			},
		},
	}
	test.WriteConfig(t, configPath, cfg)

	// without any path args
	_, err = cmd(t)
	as.NoError(err)
	assertStats(t, as, 32, 32, 32, 0)

	// specify some explicit paths
	_, err = cmd(t, "-c", "elm/elm.json", "haskell/Nested/Foo.hs")
	as.NoError(err)
	assertStats(t, as, 2, 2, 2, 0)

	// specify a bad path
	_, err = cmd(t, "-c", "elm/elm.json", "haskell/Nested/Bar.hs")
	as.ErrorContains(err, "no such file or directory")
}

func TestStdIn(t *testing.T) {
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
	out, err := cmd(t, "-C", tempDir, "--allow-missing-formatter", "--stdin")
	as.EqualError(err, "only one path should be specified when using the --stdin flag")
	as.Equal("", string(out))

	// now pass along the filename parameter
	contents = `{ foo, ... }: "hello"`
	os.Stdin = test.TempFile(t, "", "stdin", &contents)

	out, err = cmd(t, "-C", tempDir, "--allow-missing-formatter", "--stdin", "test.nix")
	as.NoError(err)
	assertStats(t, as, 1, 1, 1, 1)

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

	out, err = cmd(t, "-C", tempDir, "--allow-missing-formatter", "--stdin", "test.md")
	as.NoError(err)
	assertStats(t, as, 1, 1, 1, 1)

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

	test.WriteConfig(t, configPath, config.Config{
		Formatters: map[string]*config.Formatter{
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

	_, err = cmd(t, "-C", tempDir)
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
			idx += 1
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
	cfg := config.Config{
		Formatters: map[string]*config.Formatter{
			"echo": {
				Command:  "./echo",
				Includes: []string{"*"},
			},
		},
	}
	test.WriteConfig(t, configPath, cfg)

	// without any path args, should reformat the whole tree
	_, err = cmd(t)
	as.NoError(err)
	assertStats(t, as, 32, 32, 32, 0)

	// specify some explicit paths, relative to the elm/ sub-folder
	_, err = cmd(t, "-c", "elm.json", "../haskell/Nested/Foo.hs")
	as.NoError(err)
	assertStats(t, as, 2, 2, 2, 0)
}
