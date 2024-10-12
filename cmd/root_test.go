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

	"github.com/numtide/treefmt/cmd"

	"github.com/numtide/treefmt/config"

	"github.com/charmbracelet/log"
	"github.com/numtide/treefmt/stats"

	format2 "github.com/numtide/treefmt/cmd/format"

	"github.com/numtide/treefmt/format"

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

	_, _, err = treefmt(t, "-C", tempDir, "--allow-missing-formatter", "--on-unmatched", "fatal")
	as.ErrorContains(err, fmt.Sprintf("no formatter for path: %s", paths[0]))

	checkOutput := func(level string, output []byte) {
		for _, p := range paths {
			as.Contains(string(output), fmt.Sprintf("%s format: no formatter for path: %s", level, p))
		}
	}

	var out []byte

	// default is warn
	out, _, err = treefmt(t, "-C", tempDir, "--allow-missing-formatter", "-c")
	as.NoError(err)
	checkOutput("WARN", out)

	out, _, err = treefmt(t, "-C", tempDir, "--allow-missing-formatter", "-c", "--on-unmatched", "warn")
	as.NoError(err)
	checkOutput("WARN", out)

	out, _, err = treefmt(t, "-C", tempDir, "--allow-missing-formatter", "-c", "-u", "error")
	as.NoError(err)
	checkOutput("ERRO", out)

	out, _, err = treefmt(t, "-C", tempDir, "--allow-missing-formatter", "-c", "-v", "--on-unmatched", "info")
	as.NoError(err)
	checkOutput("INFO", out)

	t.Setenv("TREEFMT_ON_UNMATCHED", "debug")
	out, _, err = treefmt(t, "-C", tempDir, "--allow-missing-formatter", "-c", "-vv")
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
	assertStats(t, as, statz, 32, 32, 3, 3)

	setup()
	_, statz, err = treefmt(t, "-c", "--config-file", configPath, "--tree-root", tempDir, "--formatters", "elm,nix")
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 2, 2)

	setup()
	_, statz, err = treefmt(t, "-c", "--config-file", configPath, "--tree-root", tempDir, "-f", "ruby,nix")
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 2, 2)

	setup()
	_, statz, err = treefmt(t, "-c", "--config-file", configPath, "--tree-root", tempDir, "--formatters", "nix")
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 1, 1)

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
	assertStats(t, as, statz, 32, 32, 32, 0)

	// globally exclude nix files
	cfg.Excludes = []string{"*.nix"}

	test.WriteConfig(t, configPath, cfg)
	_, statz, err = treefmt(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 31, 0)

	// add haskell files to the global exclude
	cfg.Excludes = []string{"*.nix", "*.hs"}

	test.WriteConfig(t, configPath, cfg)
	_, statz, err = treefmt(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 25, 0)

	echo := cfg.FormatterConfigs["echo"]

	// remove python files from the echo formatter
	echo.Excludes = []string{"*.py"}

	test.WriteConfig(t, configPath, cfg)
	_, statz, err = treefmt(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 23, 0)

	// remove go files from the echo formatter via env
	t.Setenv("TREEFMT_FORMATTER_ECHO_EXCLUDES", "*.py,*.go")

	test.WriteConfig(t, configPath, cfg)
	_, statz, err = treefmt(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 22, 0)

	t.Setenv("TREEFMT_FORMATTER_ECHO_EXCLUDES", "") // reset

	// adjust the includes for echo to only include elm files
	echo.Includes = []string{"*.elm"}

	test.WriteConfig(t, configPath, cfg)
	_, statz, err = treefmt(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 1, 0)

	// add js files to echo formatter via env
	t.Setenv("TREEFMT_FORMATTER_ECHO_INCLUDES", "*.elm,*.js")

	test.WriteConfig(t, configPath, cfg)
	_, statz, err = treefmt(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 2, 0)
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
	assertStats(t, as, statz, 32, 32, 32, 0)
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
	assertStats(t, as, statz, 32, 32, 32, 0)

	_, statz, err = treefmt(t, "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	assertStats(t, as, statz, 32, 0, 0, 0)

	// clear cache
	_, statz, err = treefmt(t, "--config-file", configPath, "--tree-root", tempDir, "-c")
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 32, 0)

	_, statz, err = treefmt(t, "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	assertStats(t, as, statz, 32, 0, 0, 0)

	// clear cache
	_, statz, err = treefmt(t, "--config-file", configPath, "--tree-root", tempDir, "-c")
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 32, 0)

	_, statz, err = treefmt(t, "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	assertStats(t, as, statz, 32, 0, 0, 0)

	// no cache
	_, statz, err = treefmt(t, "--config-file", configPath, "--tree-root", tempDir, "--no-cache")
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
	assertStats(t, as, statz, 32, 32, 32, 0)

	// use env
	t.Setenv("TREEFMT_WORKING_DIR", tempDir)
	_, statz, err = treefmt(t, "-c")
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 32, 0)
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
	as.ErrorIs(err, format2.ErrFailOnChange)

	// we have second precision mod time tracking
	time.Sleep(time.Second)

	// test with no cache
	t.Setenv("TREEFMT_FAIL_ON_CHANGE", "true")
	test.WriteConfig(t, configPath, cfg)
	_, _, err = treefmt(t, "--config-file", configPath, "--tree-root", tempDir, "--no-cache")
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
	as.NoError(os.Setenv("PATH", binPath+":"+os.Getenv("PATH")))

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
	assertStats(t, as, statz, 32, 32, 3, 0)

	// tweak mod time of elm formatter
	as.NoError(test.RecreateSymlink(t, binPath+"/"+"elm-format"))

	_, statz, err = treefmt(t, args...)
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 3, 0)

	// check cache is working
	_, statz, err = treefmt(t, args...)
	as.NoError(err)
	assertStats(t, as, statz, 32, 0, 0, 0)

	// tweak mod time of python formatter
	as.NoError(test.RecreateSymlink(t, binPath+"/"+"black"))

	_, statz, err = treefmt(t, args...)
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 3, 0)

	// check cache is working
	_, statz, err = treefmt(t, args...)
	as.NoError(err)
	assertStats(t, as, statz, 32, 0, 0, 0)

	// add go formatter
	cfg.FormatterConfigs["go"] = &config.Formatter{
		Command:  "gofmt",
		Options:  []string{"-w"},
		Includes: []string{"*.go"},
	}
	test.WriteConfig(t, configPath, cfg)

	_, statz, err = treefmt(t, args...)
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 4, 0)

	// check cache is working
	_, statz, err = treefmt(t, args...)
	as.NoError(err)
	assertStats(t, as, statz, 32, 0, 0, 0)

	// remove python formatter
	delete(cfg.FormatterConfigs, "python")
	test.WriteConfig(t, configPath, cfg)

	_, statz, err = treefmt(t, args...)
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 2, 0)

	// check cache is working
	_, statz, err = treefmt(t, args...)
	as.NoError(err)
	assertStats(t, as, statz, 32, 0, 0, 0)

	// remove elm formatter
	delete(cfg.FormatterConfigs, "elm")
	test.WriteConfig(t, configPath, cfg)

	_, statz, err = treefmt(t, args...)
	as.NoError(err)
	assertStats(t, as, statz, 32, 32, 1, 0)

	// check cache is working
	_, statz, err = treefmt(t, args...)
	as.NoError(err)
	assertStats(t, as, statz, 32, 0, 0, 0)
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

	run := func(traversed int32, emitted int32, matched int32, formatted int32) {
		_, statz, err := treefmt(t, "-c", "--config-file", configPath, "--tree-root", tempDir)
		as.NoError(err)
		assertStats(t, as, statz, traversed, emitted, matched, formatted)
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

	// walk with filesystem instead of git
	_, statz, err := treefmt(t, "-c", "--config-file", configPath, "--tree-root", tempDir, "--walk", "filesystem")
	as.NoError(err)
	assertStats(t, as, statz, 80, 80, 80, 0)

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
	assertStats(t, as, statz, 2, 2, 2, 0)

	_, statz, err = treefmt(t, "-C", tempDir, "-c", "go", "haskell")
	as.NoError(err)
	assertStats(t, as, statz, 9, 9, 9, 0)

	_, statz, err = treefmt(t, "-C", tempDir, "-c", "go", "haskell", "ruby")
	as.NoError(err)
	assertStats(t, as, statz, 10, 10, 10, 0)

	// try with a bad path
	_, _, err = treefmt(t, "-C", tempDir, "-c", "haskell", "foo")
	as.ErrorContains(err, "path foo not found within the tree root")

	// try with a path not in the git index, e.g. it is skipped
	_, err = os.Create(filepath.Join(tempDir, "foo.txt"))
	as.NoError(err)

	_, statz, err = treefmt(t, "-C", tempDir, "-c", "haskell", "foo.txt")
	as.NoError(err)
	assertStats(t, as, statz, 8, 8, 8, 0)

	_, statz, err = treefmt(t, "-C", tempDir, "-c", "foo.txt")
	as.NoError(err)
	assertStats(t, as, statz, 1, 1, 1, 0)

	// commit
	gitCmd = exec.Command("git", "commit", "-m", "initial commit")
	gitCmd.Dir = tempDir
	as.NoError(gitCmd.Run(), "failed to commit")

	// read staged changes
	_, statz, err = treefmt(t, "-c", "--config-file", configPath, "--tree-root", tempDir, "--walk", "git_staged")
	as.NoError(err)
	assertStats(t, as, statz, 0, 0, 0, 0)

	// create some changes
	_, err = os.Create(filepath.Join(tempDir, "new.txt"))
	as.NoError(err)

	appendToFile := func(path string, content string) {
		f, err := os.OpenFile(filepath.Join(tempDir, path), os.O_APPEND|os.O_WRONLY, 0644)
		as.NoError(err)
		defer f.Close()
		_, err = f.WriteString(content)
		as.NoError(err)
	}

	appendToFile("treefmt.toml", "\n")
	appendToFile("rust/Cargo.toml", "\n")
	appendToFile("nix/sources.nix", "\n")

	// should still be empty
	_, statz, err = treefmt(t, "-c", "--config-file", configPath, "--tree-root", tempDir, "--walk", "git_staged")
	as.NoError(err)
	assertStats(t, as, statz, 0, 0, 0, 0)

	// stage changes
	gitCmd = exec.Command("git", "add", "new.txt", "treefmt.toml", "rust/Cargo.toml", "nix/sources.nix")
	gitCmd.Dir = tempDir
	as.NoError(gitCmd.Run(), "failed to stage changes")

	// read changes
	_, statz, err = treefmt(t, "-c", "--config-file", configPath, "--tree-root", tempDir, "--walk", "git_staged")
	as.NoError(err)
	assertStats(t, as, statz, 4, 4, 4, 0)

	// commit
	gitCmd = exec.Command("git", "commit", "-m", "second commit")
	gitCmd.Dir = tempDir
	as.NoError(gitCmd.Run(), "failed to commit")

	// should be empty
	_, statz, err = treefmt(t, "-c", "--config-file", configPath, "--tree-root", tempDir, "--walk", "git_staged")
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
	assertStats(t, as, statz, 32, 32, 32, 0)

	// specify some explicit paths
	_, statz, err = treefmt(t, "-c", "elm/elm.json", "haskell/Nested/Foo.hs")
	as.NoError(err)
	assertStats(t, as, statz, 2, 2, 2, 0)

	// specify a bad path
	_, _, err = treefmt(t, "-c", "elm/elm.json", "haskell/Nested/Bar.hs")
	as.ErrorContains(err, "path haskell/Nested/Bar.hs not found within the tree root")

	// specify a path outside the tree root
	externalPath := filepath.Join(cwd, "go.mod")
	_, _, err = treefmt(t, "-c", externalPath)
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
	out, _, err := treefmt(t, "-C", tempDir, "--allow-missing-formatter", "--stdin")
	as.EqualError(err, "exactly one path should be specified when using the --stdin flag")
	as.Equal("", string(out))

	// now pass along the filename parameter
	contents = `{ foo, ... }: "hello"`
	os.Stdin = test.TempFile(t, "", "stdin", &contents)

	out, statz, err := treefmt(t, "-C", tempDir, "--allow-missing-formatter", "--stdin", "test.nix")
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

	out, statz, err = treefmt(t, "-C", tempDir, "--allow-missing-formatter", "--stdin", "test.md")
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
	assertStats(t, as, statz, 32, 32, 32, 0)

	// specify some explicit paths, relative to the tree root
	_, statz, err = treefmt(t, "-c", "elm/elm.json", "haskell/Nested/Foo.hs")
	as.NoError(err)
	assertStats(t, as, statz, 2, 2, 2, 0)
}

func treefmt(t *testing.T, args ...string) ([]byte, *stats.Stats, error) {
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

func assertStats(t *testing.T, as *require.Assertions, statz *stats.Stats, traversed int32, emitted int32, matched int32, formatted int32) {
	t.Helper()
	as.Equal(traversed, statz.Value(stats.Traversed), "stats.traversed")
	as.Equal(emitted, statz.Value(stats.Emitted), "stats.emitted")
	as.Equal(matched, statz.Value(stats.Matched), "stats.matched")
	as.Equal(formatted, statz.Value(stats.Formatted), "stats.formatted")
}
