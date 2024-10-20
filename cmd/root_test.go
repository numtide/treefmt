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

	tempDir := test.TempExamples(t)

	test.ChangeWorkDir(t, tempDir)

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

	// allow missing formatter
	t.Setenv("TREEFMT_ALLOW_MISSING_FORMATTER", "true")

	checkOutput := func(level log.Level) func([]byte) {
		logPrefix := strings.ToUpper(level.String())[:4]

		return func(out []byte) {
			for _, p := range paths {
				as.Contains(string(out), fmt.Sprintf("%s no formatter for path: %s", logPrefix, p))
			}
		}
	}

	// default is WARN
	t.Run("default", func(t *testing.T) {
		treefmt2(t, withNoError(as), withOutput(checkOutput(log.WarnLevel)))
	})

	// should exit with error when using fatal
	t.Run("fatal", func(t *testing.T) {
		errorFn := func(err error) {
			as.ErrorContains(err, fmt.Sprintf("no formatter for path: %s", paths[0]))
		}

		treefmt2(t, withArgs("--on-unmatched", "fatal"), withError(errorFn))

		t.Setenv("TREEFMT_ON_UNMATCHED", "fatal")

		treefmt2(t, withError(errorFn))
	})

	// test other levels
	for _, levelStr := range []string{"debug", "info", "warn", "error"} {
		t.Run(levelStr, func(t *testing.T) {
			level, err := log.ParseLevel(levelStr)
			as.NoError(err, "failed to parse log level: %s", level)

			treefmt2(t,
				withArgs("-vv", "--on-unmatched", levelStr),
				withNoError(as),
				withOutput(checkOutput(level)),
			)

			t.Setenv("TREEFMT_ON_UNMATCHED", levelStr)

			treefmt2(t,
				withArgs("-vv"),
				withNoError(as),
				withOutput(checkOutput(level)),
			)
		})
	}

	t.Run("invalid", func(t *testing.T) {
		// test bad value
		errorFn := func(arg string) func(err error) {
			return func(err error) {
				as.ErrorContains(err, fmt.Sprintf(`invalid level: "%s"`, arg))
			}
		}

		treefmt2(t,
			withArgs("--on-unmatched", "foo"),
			withError(errorFn("foo")),
		)

		t.Setenv("TREEFMT_ON_UNMATCHED", "bar")

		treefmt2(t, withError(errorFn("bar")))
	})
}

func TestCpuProfile(t *testing.T) {
	as := require.New(t)
	tempDir := test.TempExamples(t)

	test.ChangeWorkDir(t, tempDir)

	// allow missing formatter
	t.Setenv("TREEFMT_ALLOW_MISSING_FORMATTER", "true")

	treefmt2(t,
		withArgs("--cpu-profile", "cpu.pprof"),
		withNoError(as),
	)

	as.FileExists(filepath.Join(tempDir, "cpu.pprof"))

	// test with env
	t.Setenv("TREEFMT_CPU_PROFILE", "env.pprof")

	treefmt2(t, withNoError(as))

	as.FileExists(filepath.Join(tempDir, "env.pprof"))
}

func TestAllowMissingFormatter(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)
	configPath := filepath.Join(tempDir, "treefmt.toml")

	test.ChangeWorkDir(t, tempDir)

	test.WriteConfig(t, configPath, &config.Config{
		FormatterConfigs: map[string]*config.Formatter{
			"foo-fmt": {
				Command: "foo-fmt",
			},
		},
	})

	t.Run("default", func(t *testing.T) {
		treefmt2(t,
			withError(func(err error) {
				as.ErrorIs(err, format.ErrCommandNotFound)
			}),
		)
	})

	t.Run("arg", func(t *testing.T) {
		treefmt2(t,
			withArgs("--allow-missing-formatter"),
			withNoError(as),
		)
	})

	t.Run("env", func(t *testing.T) {
		t.Setenv("TREEFMT_ALLOW_MISSING_FORMATTER", "true")
		treefmt2(t, withNoError(as))
	})
}

func TestSpecifyingFormatters(t *testing.T) {
	as := require.New(t)

	// we use the test formatter to append some whitespace
	cfg := &config.Config{
		FormatterConfigs: map[string]*config.Formatter{
			"elm": {
				Command:  "test-fmt-append",
				Options:  []string{"   "},
				Includes: []string{"*.elm"},
			},
			"nix": {
				Command:  "test-fmt-append",
				Options:  []string{"   "},
				Includes: []string{"*.nix"},
			},
			"ruby": {
				Command:  "test-fmt-append",
				Options:  []string{"   "},
				Includes: []string{"*.rb"},
			},
		},
	}

	tempDir := test.TempExamples(t)
	configPath := filepath.Join(tempDir, "treefmt.toml")

	test.WriteConfig(t, configPath, cfg)
	test.ChangeWorkDir(t, tempDir)

	t.Run("default", func(t *testing.T) {
		treefmt2(t,
			withNoError(as),
			withModtimeBump(tempDir, time.Second),
			withStats(as, map[stats.Type]int{
				stats.Traversed: 32,
				stats.Matched:   3,
				stats.Formatted: 3,
				stats.Changed:   3,
			}),
		)
	})

	t.Run("args", func(t *testing.T) {
		treefmt2(t,
			withArgs("--formatters", "elm,nix"),
			withModtimeBump(tempDir, time.Second),
			withNoError(as),
			withStats(as, map[stats.Type]int{
				stats.Traversed: 32,
				stats.Matched:   2,
				stats.Formatted: 2,
				stats.Changed:   2,
			}),
		)

		treefmt2(t,
			withArgs("--formatters", "ruby,nix"),
			withModtimeBump(tempDir, time.Second),
			withNoError(as),
			withStats(as, map[stats.Type]int{
				stats.Traversed: 32,
				stats.Matched:   2,
				stats.Formatted: 2,
				stats.Changed:   2,
			}),
		)

		treefmt2(t,
			withArgs("--formatters", "nix"),
			withModtimeBump(tempDir, time.Second),
			withNoError(as),
			withStats(as, map[stats.Type]int{
				stats.Traversed: 32,
				stats.Matched:   1,
				stats.Formatted: 1,
				stats.Changed:   1,
			}),
		)

		// bad name
		treefmt2(t,
			withArgs("--formatters", "foo"),
			withError(func(err error) {
				as.Errorf(err, "formatter not found in config: foo")
			}),
		)
	})

	t.Run("env", func(t *testing.T) {
		t.Setenv("TREEFMT_FORMATTERS", "ruby,nix")

		treefmt2(t,
			withNoError(as),
			withModtimeBump(tempDir, time.Second),
			withStats(as, map[stats.Type]int{
				stats.Traversed: 32,
				stats.Matched:   2,
				stats.Formatted: 2,
				stats.Changed:   2,
			}),
		)

		t.Setenv("TREEFMT_FORMATTERS", "bar,foo")

		treefmt2(t,
			withError(func(err error) {
				as.Errorf(err, "formatter not found in config: bar")
			}),
		)
	})
}

func TestIncludesAndExcludes(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)
	configPath := tempDir + "/treefmt.toml"

	test.ChangeWorkDir(t, tempDir)

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
	configPath := filepath.Join(tempDir, "treefmt.toml")

	t.Setenv("PRJ_ROOT", tempDir)

	treefmt2(t,
		withConfig(configPath, &config.Config{
			FormatterConfigs: map[string]*config.Formatter{
				"echo": {
					Command:  "echo",
					Includes: []string{"*"},
				},
			},
		}),
		withArgs("--config-file", configPath),
		withNoError(as),
		withStats(as, map[stats.Type]int{
			stats.Traversed: 32,
			stats.Matched:   32,
			stats.Formatted: 32,
			stats.Changed:   0,
		}),
	)
}

func TestCache(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)
	configPath := filepath.Join(tempDir, "treefmt.toml")

	test.ChangeWorkDir(t, tempDir)

	// test without any excludes
	cfg := &config.Config{
		FormatterConfigs: map[string]*config.Formatter{
			"append": {
				Command:  "test-fmt-append",
				Options:  []string{"   "},
				Includes: []string{"*"},
			},
		},
	}

	test.WriteConfig(t, configPath, cfg)

	// first run
	treefmt2(t,
		withNoError(as),
		withStats(as, map[stats.Type]int{
			stats.Traversed: 32,
			stats.Matched:   32,
			stats.Formatted: 32,
			stats.Changed:   32,
		}),
	)

	// cached run with no changes to underlying files
	treefmt2(t,
		withNoError(as),
		withStats(as, map[stats.Type]int{
			stats.Traversed: 32,
			stats.Matched:   32,
			stats.Formatted: 0,
			stats.Changed:   0,
		}),
	)

	// clear cache
	treefmt2(t,
		withArgs("-c"),
		withNoError(as),
		withStats(as, map[stats.Type]int{
			stats.Traversed: 32,
			stats.Matched:   32,
			stats.Formatted: 32,
			stats.Changed:   32,
		}),
	)

	// cached run with no changes to underlying files
	treefmt2(t,
		withNoError(as),
		withStats(as, map[stats.Type]int{
			stats.Traversed: 32,
			stats.Matched:   32,
			stats.Formatted: 0,
			stats.Changed:   0,
		}),
	)

	// bump underlying files
	treefmt2(t,
		withNoError(as),
		withModtimeBump(tempDir, time.Second),
		withStats(as, map[stats.Type]int{
			stats.Traversed: 32,
			stats.Matched:   32,
			stats.Formatted: 32,
			stats.Changed:   32,
		}),
	)

	// no cache
	treefmt2(t,
		withArgs("--no-cache"),
		withNoError(as),
		withStats(as, map[stats.Type]int{
			stats.Traversed: 32,
			stats.Matched:   32,
			stats.Formatted: 32,
			stats.Changed:   32,
		}),
	)

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

	// test that formatting errors are not cached

	// running should match but not format anything

	treefmt2(t,
		withError(func(err error) {
			as.ErrorIs(err, format.ErrFormattingFailures)
		}),
		withStats(as, map[stats.Type]int{
			stats.Traversed: 32,
			stats.Matched:   6,
			stats.Formatted: 0,
			stats.Changed:   0,
		}),
	)

	// running again should provide the same result
	treefmt2(t,
		withError(func(err error) {
			as.ErrorIs(err, format.ErrFormattingFailures)
		}),
		withStats(as, map[stats.Type]int{
			stats.Traversed: 32,
			stats.Matched:   6,
			stats.Formatted: 0,
			stats.Changed:   0,
		}),
	)

	// let's fix the haskell config so it no longer fails
	cfg.FormatterConfigs["fail"] = &config.Formatter{
		Command:  "test-fmt-append",
		Options:  []string{"   "},
		Includes: []string{"*.hs"},
	}

	test.WriteConfig(t, configPath, cfg)

	// we should now format the haskell files
	treefmt2(t,
		withNoError(as),
		withStats(as, map[stats.Type]int{
			stats.Traversed: 32,
			stats.Matched:   6,
			stats.Formatted: 6,
			stats.Changed:   6,
		}),
	)
}

func TestChangeWorkingDirectory(t *testing.T) {
	as := require.New(t)

	cfg := &config.Config{
		FormatterConfigs: map[string]*config.Formatter{
			"append": {
				Command:  "test-fmt-append",
				Options:  []string{"   "},
				Includes: []string{"*"},
			},
		},
	}

	t.Run("default", func(t *testing.T) {
		// capture current cwd, so we can replace it after the test is finished
		cwd, err := os.Getwd()
		as.NoError(err)

		t.Cleanup(func() {
			// return to the previous working directory
			as.NoError(os.Chdir(cwd))
		})

		tempDir := test.TempExamples(t)
		configPath := filepath.Join(tempDir, "treefmt.toml")

		// change to an empty temp dir and try running without specifying a working directory
		as.NoError(os.Chdir(t.TempDir()))

		treefmt2(t,
			withConfig(configPath, cfg),
			withError(func(err error) {
				as.ErrorContains(err, "failed to find treefmt config file")
			}),
		)

		// now change to the examples temp directory
		as.NoError(os.Chdir(tempDir), "failed to change to temp directory")

		treefmt2(t,
			withConfig(configPath, cfg),
			withNoError(as),
			withStats(as, map[stats.Type]int{
				stats.Traversed: 32,
			}),
		)
	})

	execute := func(t *testing.T, configFile string, env bool) {
		t.Run(configFile, func(t *testing.T) {
			// capture current cwd, so we can replace it after the test is finished
			cwd, err := os.Getwd()
			as.NoError(err)

			t.Cleanup(func() {
				// return to the previous working directory
				as.NoError(os.Chdir(cwd))
			})

			tempDir := test.TempExamples(t)
			configPath := filepath.Join(tempDir, configFile)

			// delete treefmt.toml that comes with the example folder
			as.NoError(os.Remove(filepath.Join(tempDir, "treefmt.toml")))

			var args []string

			if env {
				t.Setenv("TREEFMT_WORKING_DIR", tempDir)
			} else {
				args = []string{"-C", tempDir}
			}

			treefmt2(t,
				withArgs(args...),
				withConfig(configPath, cfg),
				withNoError(as),
				withStats(as, map[stats.Type]int{
					stats.Traversed: 32,
				}),
			)
		})
	}

	// by default, we look for a config file at ./treefmt.toml or ./.treefmt.toml in the current working directory
	configFiles := []string{"treefmt.toml", ".treefmt.toml"}

	t.Run("arg", func(t *testing.T) {
		for _, configFile := range configFiles {
			execute(t, configFile, false)
		}
	})

	t.Run("env", func(t *testing.T) {
		for _, configFile := range configFiles {
			execute(t, configFile, true)
		}
	})
}

func TestFailOnChange(t *testing.T) {
	as := require.New(t)

	t.Run("change size", func(t *testing.T) {
		tempDir := test.TempExamples(t)
		configPath := filepath.Join(tempDir, "treefmt.toml")

		cfg := &config.Config{
			FormatterConfigs: map[string]*config.Formatter{
				"append": {
					// test-fmt-append is a helper defined in nix/packages/treefmt/formatters.nix which lets us append
					// an arbitrary value to a list of files
					Command:  "test-fmt-append",
					Options:  []string{"hello"},
					Includes: []string{"elm/*"},
				},
			},
		}
		test.WriteConfig(t, configPath, cfg)

		_, statz, err := treefmt(t, "--fail-on-change", "--config-file", configPath, "--tree-root", tempDir)
		as.ErrorIs(err, formatCmd.ErrFailOnChange)

		assertStats(t, as, statz, map[stats.Type]int{
			stats.Traversed: 32,
			stats.Matched:   2,
			stats.Formatted: 2,
			stats.Changed:   2,
		})

		// cached
		_, statz, err = treefmt(t, "--fail-on-change", "--config-file", configPath, "--tree-root", tempDir)
		as.NoError(err)

		assertStats(t, as, statz, map[stats.Type]int{
			stats.Traversed: 32,
			stats.Matched:   2,
			stats.Formatted: 0,
			stats.Changed:   0,
		})
	})

	t.Run("change modtime", func(t *testing.T) {
		tempDir := test.TempExamples(t)
		configPath := filepath.Join(tempDir, "treefmt.toml")

		dateFormat := "2006 01 02 15:04.05"
		replacer := strings.NewReplacer(" ", "", ":", "")

		formatTime := func(t time.Time) string {
			// go date formats are stupid
			return replacer.Replace(t.Format(dateFormat))
		}

		writeConfig := func() {
			// new mod time is in the next second
			modTime := time.Now().Truncate(time.Second).Add(time.Second)

			cfg := &config.Config{
				FormatterConfigs: map[string]*config.Formatter{
					"append": {
						// test-fmt-modtime is a helper defined in nix/packages/treefmt/formatters.nix which lets us set
						// a file's modtime to an arbitrary date.
						// in this case, we move it forward more than a second so that our second level modtime comparison
						// will detect it as a change.
						Command:  "test-fmt-modtime",
						Options:  []string{formatTime(modTime)},
						Includes: []string{"haskell/*"},
					},
				},
			}
			test.WriteConfig(t, configPath, cfg)
		}

		writeConfig()

		_, statz, err := treefmt(t, "--fail-on-change", "--config-file", configPath, "--tree-root", tempDir)
		as.ErrorIs(err, formatCmd.ErrFailOnChange)

		assertStats(t, as, statz, map[stats.Type]int{
			stats.Traversed: 32,
			stats.Matched:   7,
			stats.Formatted: 7,
			stats.Changed:   7,
		})

		// cached
		_, statz, err = treefmt(t, "--fail-on-change", "--config-file", configPath, "--tree-root", tempDir)
		as.NoError(err)

		assertStats(t, as, statz, map[stats.Type]int{
			stats.Traversed: 32,
			stats.Matched:   7,
			stats.Formatted: 0,
			stats.Changed:   0,
		})
	})
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
				Command:  "test-fmt-append",
				Options:  []string{"fmt-a"},
				Includes: []string{"*.py"},
			},
			"fmt-b": {
				Command:  "test-fmt-append",
				Options:  []string{"fmt-b"},
				Includes: []string{"*.py"},
			},
			"fmt-c": {
				Command:  "test-fmt-append",
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

type options struct {
	args []string

	config struct {
		path  string
		value *config.Config
	}

	assertOut   func([]byte)
	assertError func(error)
	assertStats func(*stats.Stats)

	bump struct {
		path    string
		atime   time.Duration
		modtime time.Duration
	}
}

type option func(*options)

func withArgs(args ...string) option {
	return func(o *options) {
		o.args = args
	}
}

func withConfig(path string, cfg *config.Config) option {
	return func(o *options) {
		o.config.path = path
		o.config.value = cfg
	}
}

func withStats(as *require.Assertions, expected map[stats.Type]int) option {
	return func(o *options) {
		o.assertStats = func(s *stats.Stats) {
			for k, v := range expected {
				as.Equal(v, s.Value(k), k.String())
			}
		}
	}
}

func withError(fn func(error)) option {
	return func(o *options) {
		o.assertError = fn
	}
}

func withNoError(as *require.Assertions) option {
	return func(o *options) {
		o.assertError = func(err error) {
			as.NoError(err)
		}
	}
}

func withOutput(fn func([]byte)) option {
	return func(o *options) {
		o.assertOut = fn
	}
}

//nolint:unparam
func withModtimeBump(path string, bump time.Duration) option {
	return func(o *options) {
		o.bump.path = path
		o.bump.modtime = bump
	}
}

func treefmt2(
	t *testing.T,
	opt ...option,
) {
	t.Helper()

	// build options
	opts := &options{}
	for _, option := range opt {
		option(opts)
	}

	// default args if nil
	// we must pass an empty array otherwise cobra with use os.Args[1:]
	args := opts.args
	if args == nil {
		args = []string{}
	}

	// write config
	if opts.config.value != nil {
		test.WriteConfig(t, opts.config.path, opts.config.value)
	}

	// bump mod times before running
	if opts.bump.path != "" {
		test.LutimesBump(t, opts.bump.path, opts.bump.atime, opts.bump.modtime)
	}

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

	root.SetArgs(args)
	root.SetOut(tempOut)
	root.SetErr(tempOut)

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

	t.Log("\n" + string(out))

	if opts.assertStats != nil {
		opts.assertStats(statz)
	}

	if opts.assertOut != nil {
		opts.assertOut(out)
	}

	if opts.assertError != nil {
		opts.assertError(cmdErr)
	}
}
