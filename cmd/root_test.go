package cmd_test

import (
	"bufio"
	"bytes"
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
	"github.com/numtide/treefmt/v2/cmd"
	formatCmd "github.com/numtide/treefmt/v2/cmd/format"
	"github.com/numtide/treefmt/v2/config"
	"github.com/numtide/treefmt/v2/format"
	"github.com/numtide/treefmt/v2/stats"
	"github.com/numtide/treefmt/v2/test"
	"github.com/numtide/treefmt/v2/walk"
	cp "github.com/otiai10/copy"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestOnUnmatched(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)

	test.ChangeWorkDir(t, tempDir)

	expectedPaths := []string{
		".gitignore",
		"go/go.mod",
		"haskell/haskell.cabal",
		"haskell-frontend/haskell-frontend.cabal",
		"html/scripts/.gitkeep",
		"python/requirements.txt",
		// these should not be reported, they are in the global excludes
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

		regex := regexp.MustCompile(fmt.Sprintf(`^%s no formatter for path: (.*)$`, logPrefix))

		return func(out []byte) {
			var paths []string

			scanner := bufio.NewScanner(bytes.NewReader(out))
			for scanner.Scan() {
				matches := regex.FindStringSubmatch(scanner.Text())
				if len(matches) != 2 {
					continue
				}

				paths = append(paths, matches[1])
			}

			as.Equal(expectedPaths, paths)
		}
	}

	// default is INFO
	t.Run("default", func(t *testing.T) {
		treefmt(t, withArgs("-v"), withNoError(t), withStderr(checkOutput(log.InfoLevel)))
	})

	// should exit with error when using fatal
	t.Run("fatal", func(t *testing.T) {
		errorFn := func(as *require.Assertions, err error) {
			as.ErrorContains(err, "no formatter for path: "+expectedPaths[0])
		}

		treefmt(t, withArgs("--on-unmatched", "fatal"), withError(errorFn))

		t.Setenv("TREEFMT_ON_UNMATCHED", "fatal")

		treefmt(t, withError(errorFn))
	})

	// test other levels
	for _, levelStr := range []string{"debug", "info", "warn", "error"} {
		t.Run(levelStr, func(t *testing.T) {
			level, err := log.ParseLevel(levelStr)
			as.NoError(err, "failed to parse log level: %s", level)

			treefmt(t,
				withArgs("-vv", "--on-unmatched", levelStr),
				withNoError(t),
				withStderr(checkOutput(level)),
			)

			t.Setenv("TREEFMT_ON_UNMATCHED", levelStr)

			treefmt(t,
				withArgs("-vv"),
				withNoError(t),
				withStderr(checkOutput(level)),
			)
		})
	}

	t.Run("invalid", func(t *testing.T) {
		// test bad value
		errorFn := func(arg string) func(as *require.Assertions, err error) {
			return func(as *require.Assertions, err error) {
				as.ErrorContains(err, fmt.Sprintf(`invalid level: "%s"`, arg))
			}
		}

		treefmt(t,
			withArgs("--on-unmatched", "foo"),
			withError(errorFn("foo")),
		)

		t.Setenv("TREEFMT_ON_UNMATCHED", "bar")

		treefmt(t, withError(errorFn("bar")))
	})
}

func TestQuiet(t *testing.T) {
	as := require.New(t)
	tempDir := test.TempExamples(t)

	test.ChangeWorkDir(t, tempDir)

	// allow missing formatter
	t.Setenv("TREEFMT_ALLOW_MISSING_FORMATTER", "true")

	noOutput := func(out []byte) {
		as.Empty(out)
	}

	treefmt(t, withArgs("-q"), withNoError(t), withStdout(noOutput), withStderr(noOutput))
	treefmt(t, withArgs("--quiet"), withNoError(t), withStdout(noOutput), withStderr(noOutput))

	t.Setenv("TREEFMT_QUIET", "true")
	treefmt(t, withNoError(t), withStdout(noOutput), withStderr(noOutput))

	t.Setenv("TREEFMT_ALLOW_MISSING_FORMATTER", "false")

	// check it doesn't suppress errors
	treefmt(t, withError(func(as *require.Assertions, err error) {
		as.ErrorContains(err, "error looking up 'foo-fmt'")
	}))
}

func TestCpuProfile(t *testing.T) {
	as := require.New(t)
	tempDir := test.TempExamples(t)

	test.ChangeWorkDir(t, tempDir)

	// allow missing formatter
	t.Setenv("TREEFMT_ALLOW_MISSING_FORMATTER", "true")

	treefmt(t,
		withArgs("--cpu-profile", "cpu.pprof"),
		withNoError(t),
	)

	as.FileExists(filepath.Join(tempDir, "cpu.pprof"))

	// test with env
	t.Setenv("TREEFMT_CPU_PROFILE", "env.pprof")

	treefmt(t, withNoError(t))

	as.FileExists(filepath.Join(tempDir, "env.pprof"))
}

func TestAllowMissingFormatter(t *testing.T) {
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
		treefmt(t,
			withError(func(as *require.Assertions, err error) {
				as.ErrorIs(err, format.ErrCommandNotFound)
			}),
		)
	})

	t.Run("arg", func(t *testing.T) {
		treefmt(t,
			withArgs("--allow-missing-formatter"),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 33,
				stats.Matched:   0,
				stats.Formatted: 0,
				stats.Changed:   0,
			}),
		)
	})

	t.Run("env", func(t *testing.T) {
		t.Setenv("TREEFMT_ALLOW_MISSING_FORMATTER", "true")
		treefmt(t, withNoError(t))
	})
}

func TestSpecifyingFormatters(t *testing.T) {
	// we use the test formatter to append some whitespace
	cfg := &config.Config{
		FormatterConfigs: map[string]*config.Formatter{
			"rust": {
				Command:  "test-fmt-append",
				Options:  []string{"   "},
				Includes: []string{"*.rs"},
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
		treefmt(t,
			withNoError(t),
			withModtimeBump(tempDir, time.Second),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 33,
				stats.Matched:   3,
				stats.Formatted: 3,
				stats.Changed:   3,
			}),
		)
	})

	t.Run("args", func(t *testing.T) {
		treefmt(t,
			withArgs("--formatters", "rust,nix"),
			withModtimeBump(tempDir, time.Second),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 33,
				stats.Matched:   2,
				stats.Formatted: 2,
				stats.Changed:   2,
			}),
		)

		treefmt(t,
			withArgs("--formatters", "ruby,nix"),
			withModtimeBump(tempDir, time.Second),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 33,
				stats.Matched:   2,
				stats.Formatted: 2,
				stats.Changed:   2,
			}),
		)

		treefmt(t,
			withArgs("--formatters", "nix"),
			withModtimeBump(tempDir, time.Second),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 33,
				stats.Matched:   1,
				stats.Formatted: 1,
				stats.Changed:   1,
			}),
		)

		// bad name
		treefmt(t,
			withArgs("--formatters", "foo"),
			withError(func(as *require.Assertions, err error) {
				as.ErrorContains(err, "formatter foo not found in config")
			}),
		)
	})

	t.Run("env", func(t *testing.T) {
		t.Setenv("TREEFMT_FORMATTERS", "ruby,nix")

		treefmt(t,
			withNoError(t),
			withModtimeBump(tempDir, time.Second),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 33,
				stats.Matched:   2,
				stats.Formatted: 2,
				stats.Changed:   2,
			}),
		)

		t.Setenv("TREEFMT_FORMATTERS", "bar,foo")

		treefmt(t,
			withError(func(as *require.Assertions, err error) {
				as.ErrorContains(err, "formatter bar not found in config")
			}),
		)
	})

	t.Run("bad names", func(t *testing.T) {
		for _, name := range []string{"foo$", "/bar", "baz%"} {
			treefmt(t,
				withArgs("--formatters", name),
				withError(func(as *require.Assertions, err error) {
					as.ErrorContains(err, fmt.Sprintf("formatter name %q is invalid", name))
				}),
			)

			t.Setenv("TREEFMT_FORMATTERS", name)

			treefmt(t,
				withError(func(as *require.Assertions, err error) {
					as.ErrorContains(err, fmt.Sprintf("formatter name %q is invalid", name))
				}),
			)

			t.Setenv("TREEFMT_FORMATTERS", "")

			cfg.FormatterConfigs[name] = &config.Formatter{
				Command:  "echo",
				Includes: []string{"*"},
			}

			test.WriteConfig(t, configPath, cfg)

			treefmt(t,
				withError(func(as *require.Assertions, err error) {
					as.ErrorContains(err, fmt.Sprintf("formatter name %q is invalid", name))
				}),
			)

			delete(cfg.FormatterConfigs, name)

			test.WriteConfig(t, configPath, cfg)
		}
	})
}

func TestIncludesAndExcludes(t *testing.T) {
	tempDir := test.TempExamples(t)
	configPath := filepath.Join(tempDir, "treefmt.toml")

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

	treefmt(t,
		withConfig(configPath, cfg),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 33,
			stats.Matched:   33,
			stats.Formatted: 33,
			stats.Changed:   0,
		}),
	)

	// globally exclude nix files
	cfg.Excludes = []string{"*.nix"}

	treefmt(t,
		withArgs("-c"),
		withConfig(configPath, cfg),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 33,
			stats.Matched:   32,
			stats.Formatted: 32,
			stats.Changed:   0,
		}),
	)

	// add haskell files to the global exclude
	cfg.Excludes = []string{"*.nix", "*.hs"}

	treefmt(t,
		withArgs("-c"),
		withConfig(configPath, cfg),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 33,
			stats.Matched:   26,
			stats.Formatted: 26,
			stats.Changed:   0,
		}),
	)

	echo := cfg.FormatterConfigs["echo"]

	// remove python files from the echo formatter
	echo.Excludes = []string{"*.py"}

	treefmt(t,
		withArgs("-c"),
		withConfig(configPath, cfg),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 33,
			stats.Matched:   24,
			stats.Formatted: 24,
			stats.Changed:   0,
		}),
	)

	// remove go files from the echo formatter via env
	t.Setenv("TREEFMT_FORMATTER_ECHO_EXCLUDES", "*.py,*.go")

	treefmt(t,
		withArgs("-c"),
		withConfig(configPath, cfg),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 33,
			stats.Matched:   23,
			stats.Formatted: 23,
			stats.Changed:   0,
		}),
	)

	t.Setenv("TREEFMT_FORMATTER_ECHO_EXCLUDES", "") // reset

	// adjust the includes for echo to only include rust files
	echo.Includes = []string{"*.rs"}

	treefmt(t,
		withArgs("-c"),
		withConfig(configPath, cfg),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 33,
			stats.Matched:   1,
			stats.Formatted: 1,
			stats.Changed:   0,
		}),
	)

	// add js files to echo formatter via env
	t.Setenv("TREEFMT_FORMATTER_ECHO_INCLUDES", "*.rs,*.js")

	treefmt(t,
		withArgs("-c"),
		withConfig(configPath, cfg),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 33,
			stats.Matched:   2,
			stats.Formatted: 2,
			stats.Changed:   0,
		}),
	)
}

func TestConfigFile(t *testing.T) {
	as := require.New(t)

	for _, name := range []string{"treefmt.toml", ".treefmt.toml"} {
		t.Run(name, func(t *testing.T) {
			tempDir := test.TempExamples(t)

			// change to a temp directory to avoid interference with config file and auto walk detection from
			// the treefmt repository
			test.ChangeWorkDir(t, t.TempDir())

			// use a config file in a different temp directory
			configPath := filepath.Join(t.TempDir(), name)

			// if we don't specify a tree root, we default to the directory containing the config file
			treefmt(t,
				withConfig(configPath, &config.Config{
					FormatterConfigs: map[string]*config.Formatter{
						"echo": {
							Command:  "echo",
							Includes: []string{"*"},
						},
					},
				}),
				withArgs("--config-file", configPath),
				withNoError(t),
				withStats(t, map[stats.Type]int{
					stats.Traversed: 1,
					stats.Matched:   1,
					stats.Formatted: 1,
					stats.Changed:   0,
				}),
			)

			treefmt(t,
				withArgs("--config-file", configPath, "--tree-root", tempDir),
				withNoError(t),
				withStats(t, map[stats.Type]int{
					stats.Traversed: 33,
					stats.Matched:   33,
					stats.Formatted: 33,
					stats.Changed:   0,
				}),
			)

			// use env variable
			treefmt(t,
				withEnv(map[string]string{
					// TREEFMT_CONFIG takes precedence
					"TREEFMT_CONFIG": configPath,
					"PRJ_ROOT":       tempDir,
				}),
				withNoError(t),
				withStats(t, map[stats.Type]int{
					stats.Traversed: 1,
					stats.Matched:   1,
					stats.Formatted: 0,
					stats.Changed:   0,
				}),
			)

			// should fallback to PRJ_ROOT
			treefmt(t,
				withArgs("--tree-root", tempDir),
				withEnv(map[string]string{
					"PRJ_ROOT": filepath.Dir(configPath),
				}),
				withNoError(t),
				withStats(t, map[stats.Type]int{
					stats.Traversed: 33,
					stats.Matched:   33,
					stats.Formatted: 0,
					stats.Changed:   0,
				}),
			)

			// should not search upwards if using PRJ_ROOT
			configSubDir := filepath.Join(filepath.Dir(configPath), "sub")
			as.NoError(os.MkdirAll(configSubDir, 0o600))

			treefmt(t,
				withArgs("--tree-root", tempDir),
				withEnv(map[string]string{
					"PRJ_ROOT": configSubDir,
				}),
				withError(func(as *require.Assertions, err error) {
					as.ErrorContains(err, "failed to find treefmt config file")
				}),
			)
		})
	}
}

func TestCache(t *testing.T) {
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
	treefmt(t,
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 33,
			stats.Matched:   33,
			stats.Formatted: 33,
			stats.Changed:   33,
		}),
	)

	// cached run with no changes to underlying files
	treefmt(t,
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 33,
			stats.Matched:   33,
			stats.Formatted: 0,
			stats.Changed:   0,
		}),
	)

	// clear cache
	treefmt(t,
		withArgs("-c"),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 33,
			stats.Matched:   33,
			stats.Formatted: 33,
			stats.Changed:   33,
		}),
	)

	// cached run with no changes to underlying files
	treefmt(t,
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 33,
			stats.Matched:   33,
			stats.Formatted: 0,
			stats.Changed:   0,
		}),
	)

	// bump underlying files
	treefmt(t,
		withNoError(t),
		withModtimeBump(tempDir, time.Second),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 33,
			stats.Matched:   33,
			stats.Formatted: 33,
			stats.Changed:   33,
		}),
	)

	// no cache
	treefmt(t,
		withArgs("--no-cache"),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 33,
			stats.Matched:   33,
			stats.Formatted: 33,
			stats.Changed:   33,
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

	treefmt(t,
		withError(func(as *require.Assertions, err error) {
			as.ErrorIs(err, format.ErrFormattingFailures)
		}),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 33,
			stats.Matched:   6,
			stats.Formatted: 0,
			stats.Changed:   0,
		}),
	)

	// running again should provide the same result
	treefmt(t,
		withError(func(as *require.Assertions, err error) {
			as.ErrorIs(err, format.ErrFormattingFailures)
		}),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 33,
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
	treefmt(t,
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 33,
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
			//nolint:usetesting
			// return to the previous working directory
			as.NoError(os.Chdir(cwd))
		})

		tempDir := test.TempExamples(t)
		configPath := filepath.Join(tempDir, "treefmt.toml")

		//nolint:usetesting
		// change to an empty temp dir and try running without specifying a working directory
		as.NoError(os.Chdir(t.TempDir()))

		treefmt(t,
			withConfig(configPath, cfg),
			withError(func(as *require.Assertions, err error) {
				as.ErrorContains(err, "failed to find treefmt config file")
			}),
		)

		//nolint:usetesting
		// now change to the examples temp directory
		as.NoError(os.Chdir(tempDir), "failed to change to temp directory")

		treefmt(t,
			withConfig(configPath, cfg),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 33,
			}),
		)
	})

	execute := func(t *testing.T, configFile string, env bool) {
		t.Helper()
		t.Run(configFile, func(t *testing.T) {
			// capture current cwd, so we can replace it after the test is finished
			cwd, err := os.Getwd()
			as.NoError(err)

			t.Cleanup(func() {
				//nolint:usetesting
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

			treefmt(t,
				withArgs(args...),
				withConfig(configPath, cfg),
				withNoError(t),
				withStats(t, map[stats.Type]int{
					stats.Traversed: 33,
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
	t.Run("change size", func(t *testing.T) {
		tempDir := test.TempExamples(t)
		configPath := filepath.Join(tempDir, "treefmt.toml")

		test.ChangeWorkDir(t, tempDir)

		cfg := &config.Config{
			FormatterConfigs: map[string]*config.Formatter{
				"append": {
					// test-fmt-append is a helper defined in nix/packages/treefmt/formatters.nix which lets us append
					// an arbitrary value to a list of files
					Command:  "test-fmt-append",
					Options:  []string{"hello"},
					Includes: []string{"rust/*"},
				},
			},
		}

		// running with a cold cache, we should see the rust files being formatted, resulting in changes, which should
		// trigger an error
		treefmt(t,
			withArgs("--fail-on-change"),
			withConfig(configPath, cfg),
			withError(func(as *require.Assertions, err error) {
				as.ErrorIs(err, formatCmd.ErrFailOnChange)
			}),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 33,
				stats.Matched:   2,
				stats.Formatted: 2,
				stats.Changed:   2,
			}),
		)

		// running with a hot cache, we should see matches for the rust files, but no attempt to format them as the
		// underlying files have not changed since we last ran
		treefmt(t,
			withArgs("--fail-on-change"),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 33,
				stats.Matched:   2,
				stats.Formatted: 0,
				stats.Changed:   0,
			}),
		)
	})

	t.Run("change modtime", func(t *testing.T) {
		tempDir := test.TempExamples(t)
		configPath := filepath.Join(tempDir, "treefmt.toml")

		test.ChangeWorkDir(t, tempDir)

		dateFormat := "2006 01 02 15:04.05"
		replacer := strings.NewReplacer(" ", "", ":", "")

		formatTime := func(t time.Time) string {
			// go date formats are stupid
			return replacer.Replace(t.Format(dateFormat))
		}

		// running with a cold cache, we should see the haskell files being formatted, resulting in changes, which should
		// trigger an error
		treefmt(t,
			withArgs("--fail-on-change"),
			withConfigFunc(configPath, func() *config.Config {
				// new mod time is in the next second
				modTime := time.Now().Truncate(time.Second).Add(time.Second)

				return &config.Config{
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
			}),
			withError(func(as *require.Assertions, err error) {
				as.ErrorIs(err, formatCmd.ErrFailOnChange)
			}),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 33,
				stats.Matched:   7,
				stats.Formatted: 7,
				stats.Changed:   7,
			}),
		)

		// running with a hot cache, we should see matches for the haskell files, but no attempt to format them as the
		// underlying files have not changed since we last ran
		treefmt(t,
			withArgs("--fail-on-change"),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 33,
				stats.Matched:   7,
				stats.Formatted: 0,
				stats.Changed:   0,
			}),
		)
	})
}

func TestCacheBusting(t *testing.T) {
	as := require.New(t)

	t.Run("formatter_change_config", func(t *testing.T) {
		tempDir := test.TempExamples(t)
		configPath := filepath.Join(tempDir, "treefmt.toml")

		test.ChangeWorkDir(t, tempDir)

		// basic config
		cfg := &config.Config{
			FormatterConfigs: map[string]*config.Formatter{
				"python": {
					Command:  "echo", // this is non-destructive, will match but cause no changes
					Includes: []string{"*.py"},
				},
				"haskell": {
					Command:  "test-fmt-append",
					Options:  []string{"   "},
					Includes: []string{"*.hs"},
				},
			},
		}

		// initial run
		treefmt(t,
			withConfig(configPath, cfg),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 33,
				stats.Matched:   8,
				stats.Formatted: 8,
				stats.Changed:   6,
			}))

		// change formatter options
		cfg.FormatterConfigs["haskell"].Options = []string{""}

		// cache entries for haskell files should be invalidated
		treefmt(t,
			withConfig(configPath, cfg),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 33,
				stats.Matched:   8,
				stats.Formatted: 6,
				stats.Changed:   6,
			}))

		// run again, nothing should be formatted
		treefmt(t,
			withConfig(configPath, cfg),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 33,
				stats.Matched:   8,
				stats.Formatted: 0,
				stats.Changed:   0,
			}))

		// change the formatter's command
		cfg.FormatterConfigs["haskell"].Command = "echo"

		// cache entries for haskell files should be invalidated
		treefmt(t,
			withConfig(configPath, cfg),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 33,
				stats.Matched:   8,
				stats.Formatted: 6,
				stats.Changed:   0, // echo doesn't affect the files so no changes expected
			}))

		// run again, nothing should be formatted
		treefmt(t,
			withConfig(configPath, cfg),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 33,
				stats.Matched:   8,
				stats.Formatted: 0,
				stats.Changed:   0,
			}))

		// change the formatters includes
		cfg.FormatterConfigs["haskell"].Includes = []string{"haskell/*.hs"}

		// we should match on fewer files, but no formatting should occur as includes are not part of the formatting
		// signature
		treefmt(t,
			withConfig(configPath, cfg),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 33,
				stats.Matched:   6,
				stats.Formatted: 0,
				stats.Changed:   0,
			}))

		// change the formatters excludes
		cfg.FormatterConfigs["haskell"].Excludes = []string{"haskell/Foo.hs"}

		// we should match on fewer files, but no formatting should occur as excludes are not part of the formatting
		// signature
		treefmt(t,
			withConfig(configPath, cfg),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 33,
				stats.Matched:   5,
				stats.Formatted: 0,
				stats.Changed:   0,
			}))
	})

	t.Run("formatter_change_binary", func(t *testing.T) {
		tempDir := test.TempExamples(t)
		configPath := filepath.Join(tempDir, "treefmt.toml")

		test.ChangeWorkDir(t, tempDir)

		// find test-fmt-append in PATH
		sourcePath, err := exec.LookPath("test-fmt-append")
		as.NoError(err, "failed to find test-fmt-append in PATH")

		// copy it into the temp dir so we can mess with its size and modtime
		binPath := filepath.Join(tempDir, "bin")
		as.NoError(os.Mkdir(binPath, 0o755))

		scriptPath := filepath.Join(binPath, "test-fmt-append")
		as.NoError(cp.Copy(sourcePath, scriptPath, cp.Options{AddPermission: 0o755}))

		// prepend our test bin directory to PATH
		t.Setenv("PATH", binPath+":"+os.Getenv("PATH"))

		// basic config
		cfg := &config.Config{
			FormatterConfigs: map[string]*config.Formatter{
				"python": {
					Command:  "echo", // this is non-destructive, will match but cause no changes
					Includes: []string{"*.py"},
				},
				"rust": {
					Command:  "test-fmt-append",
					Options:  []string{"   "},
					Includes: []string{"*.rs"},
				},
			},
		}

		// initial run
		treefmt(t,
			withConfig(configPath, cfg),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 34,
				stats.Matched:   3,
				stats.Formatted: 3,
				stats.Changed:   1,
			}))

		// tweak mod time of rust formatter
		newTime := time.Now().Add(-time.Minute)
		as.NoError(os.Chtimes(scriptPath, newTime, newTime))

		// cache entries for rust files should be invalidated
		treefmt(t,
			withConfig(configPath, cfg),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 34,
				stats.Matched:   3,
				stats.Formatted: 1,
				stats.Changed:   1,
			}))

		// running again with a hot cache, we should see nothing be formatted
		treefmt(t,
			withConfig(configPath, cfg),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 34,
				stats.Matched:   3,
				stats.Formatted: 0,
				stats.Changed:   0,
			}),
		)

		// tweak the size of rust formatter
		formatter, err := os.OpenFile(scriptPath, os.O_WRONLY|os.O_APPEND, 0o755)
		as.NoError(err, "failed to open rust formatter")

		_, err = formatter.WriteString(" ") // add some whitespace
		as.NoError(err, "failed to append to rust formatter")
		as.NoError(formatter.Close(), "failed to close rust formatter")

		// cache entries for rust files should be invalidated
		treefmt(t,
			withConfig(configPath, cfg),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 34,
				stats.Matched:   3,
				stats.Formatted: 1,
				stats.Changed:   1,
			}))

		// running again with a hot cache, we should see nothing be formatted
		treefmt(t,
			withConfig(configPath, cfg),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 34,
				stats.Matched:   3,
				stats.Formatted: 0,
				stats.Changed:   0,
			}),
		)
	})

	t.Run("formatter_add_remove", func(t *testing.T) {
		tempDir := test.TempExamples(t)
		configPath := filepath.Join(tempDir, "treefmt.toml")

		test.ChangeWorkDir(t, tempDir)

		cfg := &config.Config{
			FormatterConfigs: map[string]*config.Formatter{
				"python": {
					Command:  "test-fmt-append",
					Options:  []string{"   "},
					Includes: []string{"*.py"},
				},
			},
		}

		// initial run
		treefmt(t,
			withConfig(configPath, cfg),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 33,
				stats.Matched:   2,
				stats.Formatted: 2,
				stats.Changed:   2,
			}),
		)

		// cached run
		treefmt(t,
			withConfig(configPath, cfg),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 33,
				stats.Matched:   2,
				stats.Formatted: 0,
				stats.Changed:   0,
			}),
		)

		// add a formatter
		cfg.FormatterConfigs["rust"] = &config.Formatter{
			Command:  "test-fmt-append",
			Options:  []string{"   "},
			Includes: []string{"*.rs"},
		}

		// only the rust files should be formatted
		treefmt(t,
			withConfig(configPath, cfg),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 33,
				stats.Matched:   3,
				stats.Formatted: 1,
				stats.Changed:   1,
			}),
		)

		// let's add a second python formatter
		cfg.FormatterConfigs["python_secondary"] = &config.Formatter{
			Command:  "test-fmt-append",
			Options:  []string{" "},
			Includes: []string{"*.py"},
		}

		// python files should be formatted as their pipeline has changed
		treefmt(t,
			withConfig(configPath, cfg),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 33,
				stats.Matched:   3,
				stats.Formatted: 2,
				stats.Changed:   2,
			}),
		)

		// cached run
		treefmt(t,
			withConfig(configPath, cfg),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 33,
				stats.Matched:   3,
				stats.Formatted: 0,
				stats.Changed:   0,
			}),
		)

		// change ordering within a pipeline
		cfg.FormatterConfigs["python"].Priority = 2
		cfg.FormatterConfigs["python_secondary"].Priority = 1

		// python files should be formatted as their pipeline has changed
		treefmt(t,
			withConfig(configPath, cfg),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 33,
				stats.Matched:   3,
				stats.Formatted: 2,
				stats.Changed:   2,
			}),
		)

		// cached run
		treefmt(t,
			withConfig(configPath, cfg),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 33,
				stats.Matched:   3,
				stats.Formatted: 0,
				stats.Changed:   0,
			}),
		)

		// remove secondary python formatter
		delete(cfg.FormatterConfigs, "python_secondary")

		// python files should be formatted as their pipeline has changed
		treefmt(t,
			withConfig(configPath, cfg),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 33,
				stats.Matched:   3,
				stats.Formatted: 2,
				stats.Changed:   2,
			}),
		)

		// cached run
		treefmt(t,
			withConfig(configPath, cfg),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 33,
				stats.Matched:   3,
				stats.Formatted: 0,
				stats.Changed:   0,
			}),
		)

		// remove the rust formatter
		delete(cfg.FormatterConfigs, "rust")

		// only python files should match, but no formatting should occur as not formatting signatures have been
		// affected
		treefmt(t,
			withConfig(configPath, cfg),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 33,
				stats.Matched:   2,
				stats.Formatted: 0,
				stats.Changed:   0,
			}),
		)
	})
}

func TestGit(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)
	configPath := filepath.Join(tempDir, "/treefmt.toml")

	test.ChangeWorkDir(t, tempDir)

	// basic config
	cfg := &config.Config{
		FormatterConfigs: map[string]*config.Formatter{
			"echo": {
				Command:  "echo", // will not generate any underlying changes in the file
				Includes: []string{"*"},
			},
		},
	}

	test.WriteConfig(t, configPath, cfg)

	// init a git repo
	gitCmd := exec.Command("git", "init")
	as.NoError(gitCmd.Run(), "failed to init git repository")

	// run before adding anything to the index
	// we should pick up untracked files since we use `git ls-files -o`
	treefmt(t,
		withConfig(configPath, cfg),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 33,
			stats.Matched:   33,
			stats.Formatted: 33,
			stats.Changed:   0,
		}),
	)

	// add everything to the index
	gitCmd = exec.Command("git", "add", ".")
	as.NoError(gitCmd.Run(), "failed to add everything to the index")

	treefmt(t,
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 33,
			stats.Matched:   33,
			stats.Formatted: 0,
			stats.Changed:   0,
		}),
	)

	// create a file which should be in .gitignore
	f, err := os.CreateTemp(tempDir, "test-*.txt")
	as.NoError(err, "failed to create temp file")

	t.Cleanup(func() {
		_ = f.Close()
	})

	treefmt(t,
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 33,
			stats.Matched:   33,
			stats.Formatted: 0,
			stats.Changed:   0,
		}),
	)

	// remove python directory
	as.NoError(os.RemoveAll(filepath.Join(tempDir, "python")), "failed to remove python directory")

	// we should traverse and match against fewer files, but no formatting should occur as no formatting signatures
	// are impacted
	treefmt(t,
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 30,
			stats.Matched:   30,
			stats.Formatted: 0,
			stats.Changed:   0,
		}),
	)

	// remove nixpkgs.toml from the filesystem but leave it in the index
	as.NoError(os.Remove(filepath.Join(tempDir, "nixpkgs.toml")))

	// walk with filesystem instead of with git
	// the .git folder contains 50 additional files
	// when added to the 30 we started with (34 minus nixpkgs.toml which we removed from the filesystem), we should
	// traverse 82 files.
	treefmt(t,
		withArgs("--walk", "filesystem"),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 82,
			stats.Matched:   82,
			stats.Formatted: 53, // the echo formatter should only be applied to the new files
			stats.Changed:   0,
		}),
	)

	// format specific sub paths
	// we should traverse and match against those files, but without any underlying change to their files or their
	// formatting config, we will not format them

	treefmt(t,
		withArgs("go"),
		withConfig(configPath, cfg),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 2,
			stats.Matched:   2,
			stats.Formatted: 0,
			stats.Changed:   0,
		}),
	)

	treefmt(t,
		withArgs("go", "haskell"),
		withConfig(configPath, cfg),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 9,
			stats.Matched:   9,
			stats.Formatted: 0,
			stats.Changed:   0,
		}),
	)

	treefmt(t,
		withArgs("-C", tempDir, "go", "haskell", "ruby"),
		withConfig(configPath, cfg),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 10,
			stats.Matched:   10,
			stats.Formatted: 0,
			stats.Changed:   0,
		}),
	)

	// try with a bad path
	treefmt(t,
		withArgs("-C", tempDir, "haskell", "foo"),
		withConfig(configPath, cfg),
		withError(func(as *require.Assertions, err error) {
			as.ErrorContains(err, "foo not found")
		}),
	)

	// try with a path not in the git index
	_, err = os.Create(filepath.Join(tempDir, "foo.txt"))
	as.NoError(err)

	treefmt(t,
		withArgs("haskell", "foo.txt", "-vv"),
		withConfig(configPath, cfg),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 8,
			stats.Matched:   8,
			stats.Formatted: 1, // we only format foo.txt, which is new to the cache
			stats.Changed:   0,
		}),
	)

	treefmt(t,
		withArgs("go", "foo.txt"),
		withConfig(configPath, cfg),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 3,
			stats.Matched:   3,
			stats.Formatted: 0,
			stats.Changed:   0,
		}),
	)

	treefmt(t,
		withArgs("foo.txt"),
		withConfig(configPath, cfg),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 1,
			stats.Matched:   1,
			stats.Formatted: 0,
			stats.Changed:   0,
		}),
	)
}

func TestJujutsu(t *testing.T) {
	as := require.New(t)

	test.SetenvXdgConfigDir(t)
	tempDir := test.TempExamples(t)
	configPath := filepath.Join(tempDir, "/treefmt.toml")

	test.ChangeWorkDir(t, tempDir)

	// basic config
	cfg := &config.Config{
		FormatterConfigs: map[string]*config.Formatter{
			"echo": {
				Command:  "echo", // will not generate any underlying changes in the file
				Includes: []string{"*"},
			},
		},
	}

	test.WriteConfig(t, configPath, cfg)

	// init a jujutsu repo
	jjCmd := exec.Command("jj", "git", "init")
	as.NoError(jjCmd.Run(), "failed to init jujutsu repository")

	// run treefmt before adding anything to the jj index
	// Jujutsu depends on updating the index with a `jj` command. So, until we do
	// that, the treefmt should return nothing, since the walker is executed with
	// `--ignore-working-copy` which does not update the index.
	treefmt(t,
		withConfig(configPath, cfg),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 0,
			stats.Matched:   0,
			stats.Formatted: 0,
			stats.Changed:   0,
		}),
	)

	// update jujutsu's index
	jjCmd = exec.Command("jj")
	as.NoError(jjCmd.Run(), "failed to update the index")

	// This is our first pass, since previously the files were not in the index. This should format all files.
	treefmt(t,
		withConfig(configPath, cfg),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 33,
			stats.Matched:   33,
			stats.Formatted: 33,
			stats.Changed:   0,
		}),
	)

	// create a file which should be in .gitignore
	f, err := os.CreateTemp(tempDir, "test-*.txt")
	as.NoError(err, "failed to create temp file")

	// update jujutsu's index
	jjCmd = exec.Command("jj")
	as.NoError(jjCmd.Run(), "failed to update the index")

	t.Cleanup(func() {
		_ = f.Close()
	})

	treefmt(t,
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 33,
			stats.Matched:   33,
			stats.Formatted: 0,
			stats.Changed:   0,
		}),
	)

	// remove python directory
	as.NoError(os.RemoveAll(filepath.Join(tempDir, "python")), "failed to remove python directory")

	// update jujutsu's index
	jjCmd = exec.Command("jj")
	as.NoError(jjCmd.Run(), "failed to update the index")

	// we should traverse and match against fewer files, but no formatting should occur as no formatting signatures
	// are impacted
	treefmt(t,
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 30,
			stats.Matched:   30,
			stats.Formatted: 0,
			stats.Changed:   0,
		}),
	)

	// remove nixpkgs.toml from the filesystem but leave it in the index
	as.NoError(os.Remove(filepath.Join(tempDir, "nixpkgs.toml")))

	// walk with filesystem instead of with jujutsu
	// the .jj folder contains 100 additional files
	// when added to the 30 we started with (34 minus nixpkgs.toml which we removed from the filesystem), we should
	// traverse 130 files.
	treefmt(t,
		withArgs("--walk", "filesystem"),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 133,
			stats.Matched:   133,
			stats.Formatted: 104, // the echo formatter should only be applied to the new files
			stats.Changed:   0,
		}),
	)

	// format specific sub paths
	// we should traverse and match against those files, but without any underlying change to their files or their
	// formatting config, we will not format them

	treefmt(t,
		withArgs("go"),
		withConfig(configPath, cfg),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 2,
			stats.Matched:   2,
			stats.Formatted: 0,
			stats.Changed:   0,
		}),
	)

	treefmt(t,
		withArgs("go", "haskell"),
		withConfig(configPath, cfg),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 9,
			stats.Matched:   9,
			stats.Formatted: 0,
			stats.Changed:   0,
		}),
	)

	treefmt(t,
		withArgs("-C", tempDir, "go", "haskell", "ruby"),
		withConfig(configPath, cfg),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 10,
			stats.Matched:   10,
			stats.Formatted: 0,
			stats.Changed:   0,
		}),
	)

	// try with a bad path
	treefmt(t,
		withArgs("-C", tempDir, "haskell", "foo"),
		withConfig(configPath, cfg),
		withError(func(as *require.Assertions, err error) {
			as.ErrorContains(err, "foo not found")
		}),
	)

	// try with a path not in the jj index
	_, err = os.Create(filepath.Join(tempDir, "foo.txt"))
	as.NoError(err)

	// update jujutsu's index
	jjCmd = exec.Command("jj")
	as.NoError(jjCmd.Run(), "failed to update the index")

	treefmt(t,
		withArgs("haskell", "foo.txt", "-vv"),
		withConfig(configPath, cfg),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 8,
			stats.Matched:   8,
			stats.Formatted: 1, // we only format foo.txt, which is new to the cache
			stats.Changed:   0,
		}),
	)

	treefmt(t,
		withArgs("go", "foo.txt"),
		withConfig(configPath, cfg),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 3,
			stats.Matched:   3,
			stats.Formatted: 0,
			stats.Changed:   0,
		}),
	)

	treefmt(t,
		withArgs("foo.txt"),
		withConfig(configPath, cfg),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 1,
			stats.Matched:   1,
			stats.Formatted: 0,
			stats.Changed:   0,
		}),
	)
}

func TestTreeRootCmd(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)
	configPath := filepath.Join(tempDir, "/treefmt.toml")

	test.ChangeWorkDir(t, tempDir)

	// basic config
	cfg := &config.Config{
		FormatterConfigs: map[string]*config.Formatter{
			"echo": {
				Command:  "echo", // will not generate any underlying changes in the file
				Includes: []string{"*"},
			},
		},
	}

	test.WriteConfig(t, configPath, cfg)

	// construct a tree root command with some error logging and dumping output on stdout
	treeRootCmd := func(output string) string {
		return fmt.Sprintf("bash -c '>&2 echo -e \"some error text\nsome more error text\" && echo %s'", output)
	}

	// helper for checking the contents of stderr matches our expected debug output
	checkStderr := func(buf []byte) {
		output := string(buf)
		as.Contains(output, "DEBU tree-root-cmd | stderr: some error text\n")
		as.Contains(output, "DEBU tree-root-cmd | stderr: some more error text\n")
	}

	// run treefmt with DEBUG logging enabled and with tree root cmd being the root of the temp directory
	treefmt(t,
		withArgs("-vv", "--tree-root-cmd", treeRootCmd(tempDir)),
		withNoError(t),
		withStderr(checkStderr),
		withConfig(configPath, cfg),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 33,
			stats.Matched:   33,
			stats.Formatted: 33,
			stats.Changed:   0,
		}),
	)

	// run from a subdirectory, mixing things up by specifying the command via an env variable
	treefmt(t,
		withArgs("-vv"),
		withEnv(map[string]string{
			"TREEFMT_TREE_ROOT_CMD": treeRootCmd(filepath.Join(tempDir, "go")),
		}),
		withNoError(t),
		withStderr(checkStderr),
		withConfig(configPath, cfg),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 2,
			stats.Matched:   2,
			stats.Formatted: 2,
			stats.Changed:   0,
		}),
	)

	// run from a subdirectory, mixing things up by specifying the command via config
	cfg.TreeRootCmd = treeRootCmd(filepath.Join(tempDir, "haskell"))

	treefmt(t,
		withArgs("-vv"),
		withNoError(t),
		withStderr(checkStderr),
		withConfig(configPath, cfg),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 7,
			stats.Matched:   7,
			stats.Formatted: 7,
			stats.Changed:   0,
		}),
	)

	// run with a long-running command (2 seconds or more)
	treefmt(t,
		withArgs(
			"-vv",
			"--tree-root-cmd", fmt.Sprintf(
				"bash -c 'sleep 2 && echo %s'",
				tempDir,
			),
		),
		withError(func(as *require.Assertions, err error) {
			as.ErrorContains(err, "tree-root-cmd was killed after taking more than 2s to execute")
		}),
		withConfig(configPath, cfg),
	)

	// run with a command that outputs multiple lines
	treefmt(t,
		withArgs(
			"--tree-root-cmd", fmt.Sprintf(
				"bash -c 'echo %s && echo %s'",
				tempDir, tempDir,
			),
		),
		withStderr(func(buf []byte) {
			as.Contains(string(buf), fmt.Sprintf("ERRO tree-root-cmd | stdout: \n%s\n%s\n", tempDir, tempDir))
		}),
		withError(func(as *require.Assertions, err error) {
			as.ErrorContains(err, "tree-root-cmd cannot output multiple lines")
		}),
		withConfig(configPath, cfg),
	)
}

func TestTreeRootExclusivity(t *testing.T) {
	tempDir := test.TempExamples(t)
	configPath := filepath.Join(tempDir, "/treefmt.toml")

	formatterConfigs := map[string]*config.Formatter{
		"echo": {
			Command:  "echo", // will not generate any underlying changes in the file
			Includes: []string{"*"},
		},
	}

	test.ChangeWorkDir(t, tempDir)

	assertExclusiveFlag := func(as *require.Assertions, err error) {
		as.ErrorContains(err,
			"if any flags in the group [tree-root tree-root-cmd tree-root-file] are set none of the others can be;",
		)
	}

	assertExclusiveConfig := func(as *require.Assertions, err error) {
		as.ErrorContains(err,
			"at most one of tree-root, tree-root-cmd or tree-root-file can be specified",
		)
	}

	envValues := map[string][]string{
		"tree-root":      {"TREEFMT_TREE_ROOT", "bar"},
		"tree-root-cmd":  {"TREEFMT_TREE_ROOT_CMD", "echo /foo/bar"},
		"tree-root-file": {"TREEFMT_TREE_ROOT_FILE", ".git/config"},
	}

	flagValues := map[string][]string{
		"tree-root":      {"--tree-root", "bar"},
		"tree-root-cmd":  {"--tree-root-cmd", "'echo /foo/bar'"},
		"tree-root-file": {"--tree-root-file", ".git/config"},
	}

	configValues := map[string]func(*config.Config){
		"tree-root": func(cfg *config.Config) {
			cfg.TreeRoot = "bar"
		},
		"tree-root-cmd": func(cfg *config.Config) {
			cfg.TreeRootCmd = "'echo /foo/bar'"
		},
		"tree-root-file": func(cfg *config.Config) {
			cfg.TreeRootFile = ".git/config"
		},
	}

	invalidCombinations := [][]string{
		{"tree-root", "tree-root-cmd"},
		{"tree-root", "tree-root-file"},
		{"tree-root-cmd", "tree-root-file"},
		{"tree-root", "tree-root-cmd", "tree-root-file"},
	}

	// TODO we should also test mixing the various methods in the same test e.g. env variable and config value
	// Given that ultimately everything is being reduced into the config object after parsing from viper, I'm fairly
	// confident if these tests all pass then the mixed methods should yield the same result.

	// for each set of invalid args, test them with flags, environment variables, and config entries.
	for _, combination := range invalidCombinations {
		// test flags
		var args []string
		for _, key := range combination {
			args = append(args, flagValues[key]...)
		}

		treefmt(t,
			withArgs(args...),
			withError(assertExclusiveFlag),
		)

		// test env variables
		env := make(map[string]string)

		for _, key := range combination {
			entry := envValues[key]
			env[entry[0]] = entry[1]
		}

		treefmt(t,
			withEnv(env),
			withError(assertExclusiveConfig),
		)

		// test config
		cfg := &config.Config{
			FormatterConfigs: formatterConfigs,
		}

		for _, key := range combination {
			entry := configValues[key]
			entry(cfg)
		}

		treefmt(t,
			withConfig(configPath, cfg),
			withError(assertExclusiveConfig),
		)
	}
}

func TestPathsArg(t *testing.T) {
	as := require.New(t)

	// capture current cwd, so we can replace it after the test is finished
	cwd, err := os.Getwd()
	as.NoError(err)

	t.Cleanup(func() {
		//nolint:usetesting
		// return to the previous working directory
		as.NoError(os.Chdir(cwd))
	})

	// create a project root under a temp dir to verify behaviour with files inside the temp dir, but outside the
	// project root
	tempDir := t.TempDir()
	treeRoot := filepath.Join(tempDir, "tree-root")

	test.TempExamplesInDir(t, treeRoot)

	configPath := filepath.Join(treeRoot, "/treefmt.toml")

	// create a file outside the treeRoot
	externalFile, err := os.Create(filepath.Join(tempDir, "outside_tree.go"))
	as.NoError(err)

	//nolint:usetesting
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
	treefmt(t,
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 33,
			stats.Matched:   33,
			stats.Formatted: 33,
			stats.Changed:   0,
		}),
	)

	// specify some explicit paths
	treefmt(t,
		withArgs("rust/src/main.rs", "haskell/Nested/Foo.hs"),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 2,
			stats.Matched:   2,
			stats.Formatted: 0,
			stats.Changed:   0,
		}),
	)

	// specify an absolute path
	absoluteInternalPath, err := filepath.Abs("rust/src/main.rs")
	as.NoError(err)

	treefmt(t,
		withArgs(absoluteInternalPath),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 1,
			stats.Matched:   1,
			stats.Formatted: 0,
			stats.Changed:   0,
		}),
	)

	// specify a bad path
	treefmt(t,
		withArgs("rust/src/main.rs", "haskell/Nested/Bar.hs"),
		withError(func(as *require.Assertions, err error) {
			as.ErrorContains(err, "Bar.hs not found")
		}),
	)

	// specify an absolute path outside the tree root
	absoluteExternalPath, err := filepath.Abs(externalFile.Name())
	as.NoError(err)
	as.FileExists(absoluteExternalPath, "external file must exist")

	treefmt(t,
		withArgs(absoluteExternalPath),
		withError(func(as *require.Assertions, err error) {
			as.ErrorContains(err, fmt.Sprintf("path %s not inside the tree root", absoluteExternalPath))
		}),
	)

	// specify a relative path outside the tree root
	relativeExternalPath := "../outside_tree.go"
	as.FileExists(relativeExternalPath, "external file must exist")

	treefmt(t,
		withArgs(relativeExternalPath),
		withError(func(as *require.Assertions, err error) {
			as.ErrorContains(err, fmt.Sprintf("path %s not inside the tree root", relativeExternalPath))
		}),
	)
}

func TestStdin(t *testing.T) {
	as := require.New(t)
	tempDir := test.TempExamples(t)

	test.ChangeWorkDir(t, tempDir)

	// capture current stdin and replace it on test cleanup
	prevStdIn := os.Stdin

	t.Cleanup(func() {
		os.Stdin = prevStdIn
	})

	// omit the required filename parameter
	contents := `{ foo, ... }: "hello"`
	os.Stdin = test.TempFile(t, "", "stdin", &contents)

	// for convenience so we don't have to specify it in the args
	t.Setenv("TREEFMT_ALLOW_MISSING_FORMATTER", "true")

	// we get an error about the missing filename parameter.
	treefmt(t,
		withArgs("--stdin"),
		withError(func(as *require.Assertions, err error) {
			as.EqualError(err, "exactly one path should be specified when using the --stdin flag")
		}),
		withStderr(func(out []byte) {
			as.Equal("Error: exactly one path should be specified when using the --stdin flag\n", string(out))
		}),
	)

	// now pass along the filename parameter
	os.Stdin = test.TempFile(t, "", "stdin", &contents)

	treefmt(t,
		withArgs("--stdin", "test.nix"),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 1,
			stats.Matched:   1,
			stats.Formatted: 1,
			stats.Changed:   1,
		}),
		withStdout(func(out []byte) {
			as.Equal(`{ ...}: "hello"
`, string(out))
		}),
	)

	// the nix formatters should have reduced the example to the following

	// try a file that's outside of the project root
	os.Stdin = test.TempFile(t, "", "stdin", &contents)

	treefmt(t,
		withArgs("--stdin", "../test.nix"),
		withError(func(as *require.Assertions, err error) {
			as.ErrorContains(err, "path ../test.nix not inside the tree root "+tempDir)
		}),
		withStderr(func(out []byte) {
			as.Contains(string(out), "Error: failed to create walker: path ../test.nix not inside the tree root")
		}),
	)

	// try some markdown instead
	contents = `
| col1 | col2 |
| ---- | ---- |
| nice | fits |
| oh no! | it's ugly |
`
	os.Stdin = test.TempFile(t, "", "stdin", &contents)

	treefmt(t,
		withArgs("--stdin", "test.md"),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 1,
			stats.Matched:   1,
			stats.Formatted: 1,
			stats.Changed:   1,
		}),
		withStdout(func(out []byte) {
			as.Equal(`| col1   | col2      |
| ------ | --------- |
| nice   | fits      |
| oh no! | it's ugly |
`, string(out))
		}),
	)

	// try with a justfile and a path which doesn't exist within the project root
	contents = `
# print this message
help:
        just --list --list-submodules --unsorted

`
	os.Stdin = test.TempFile(t, "", "stdin", &contents)

	treefmt(t,
		withArgs("--stdin", "foo/justfile"),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 1,
			stats.Matched:   1,
			stats.Formatted: 1,
			stats.Changed:   1,
		}),
		withStdout(func(out []byte) {
			as.Equal(`# print this message
help:
    just --list --list-submodules --unsorted
`, string(out))
		}),
	)
}

func TestDeterministicOrderingInPipeline(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)
	configPath := tempDir + "/treefmt.toml"

	test.ChangeWorkDir(t, tempDir)

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

	treefmt(t, withNoError(t))

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
			tempDir := test.TempExamples(t)
			configPath := filepath.Join(tempDir, "/treefmt.toml")

			test.ChangeWorkDir(t, tempDir)

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

			//nolint:usetesting
			// change working directory to subdirectory
			as.NoError(os.Chdir(filepath.Join(tempDir, "go")))

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
			treefmt(t,
				withNoError(t),
				withStats(t, map[stats.Type]int{
					stats.Traversed: 33,
					stats.Matched:   33,
					stats.Formatted: 33,
					stats.Changed:   0,
				}),
			)

			// specify some explicit paths, relative to the tree root
			// this should not work, as we're in a subdirectory
			treefmt(t,
				withArgs("-c", "go/main.go", "haskell/Nested/Foo.hs"),
				withError(func(as *require.Assertions, err error) {
					as.ErrorContains(err, "go/main.go not found")
				}),
			)

			// specify some explicit paths, relative to the current directory
			treefmt(t,
				withArgs("-c", "main.go", "../haskell/Nested/Foo.hs"),
				withNoError(t),
				withStats(t, map[stats.Type]int{
					stats.Traversed: 2,
					stats.Matched:   2,
					stats.Formatted: 2,
					stats.Changed:   0,
				}),
			)
		})
	}
}

// Check that supplying paths on the command-line works when an element of the
// project root is a symlink.
//
// Regression test for #578.
//
// See: https://github.com/numtide/treefmt/issues/578
func TestProjectRootIsSymlink(t *testing.T) {
	as := require.New(t)

	tempDir := t.TempDir()
	realRoot := filepath.Join(tempDir, "/real-root")
	test.TempExamplesInDir(t, realRoot)

	symlinkRoot := filepath.Join(tempDir, "/project-root")
	err := os.Symlink(realRoot, symlinkRoot)
	as.NoError(err)

	test.ChangeWorkDir(t, symlinkRoot)

	// basic config
	cfg := &config.Config{
		FormatterConfigs: map[string]*config.Formatter{
			"echo": {
				Command:  "echo",
				Includes: []string{"*"},
			},
		},
	}

	configPath := filepath.Join(symlinkRoot, "/treefmt.toml")
	test.WriteConfig(t, configPath, cfg)

	// Verify we can format a specific file.
	treefmt(t,
		withArgs("-c", "go/main.go"),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 1,
			stats.Matched:   1,
			stats.Formatted: 1,
			stats.Changed:   0,
		}),
	)

	// Verify we can format a specific directory that is a symlink.
	treefmt(t,
		withArgs("-c", "symlink-to-yaml-dir"),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 1,
			stats.Matched:   1,
			stats.Formatted: 1,
			stats.Changed:   0,
		}),
	)

	// Verify we can format the current directory (which is a symlink!).
	treefmt(t,
		withArgs("-c", "."),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 33,
			stats.Matched:   33,
			stats.Formatted: 33,
			stats.Changed:   0,
		}),
	)
}

func TestConcurrentInvocation(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)
	configPath := filepath.Join(tempDir, "/treefmt.toml")

	test.ChangeWorkDir(t, tempDir)

	cfg := &config.Config{
		FormatterConfigs: map[string]*config.Formatter{
			"echo": {
				Command:  "echo",
				Includes: []string{"*"},
			},
			"slow": {
				Command: "test-fmt-delayed-append",
				// connect timeout for the db is 1 second
				// wait 2 seconds before appending ' ' to each provided path
				Options:  []string{"2", " "},
				Includes: []string{"*"},
			},
		},
	}

	eg := errgroup.Group{}

	// concurrent invocation with one slow instance and one not

	eg.Go(func() error {
		treefmt(t,
			withArgs("--formatters", "slow"),
			withConfig(configPath, cfg),
			withNoError(t),
		)

		return nil
	})

	time.Sleep(500 * time.Millisecond)

	treefmt(t,
		withArgs("--formatters", "echo"),
		withConfig(configPath, cfg),
		withError(func(as *require.Assertions, err error) {
			as.ErrorContains(err, "failed to open cache")
		}),
	)

	as.NoError(eg.Wait())

	// concurrent invocation with one slow instance and one configured to clear the cache

	eg.Go(func() error {
		treefmt(t,
			withArgs("--formatters", "slow"),
			withConfig(configPath, cfg),
			withNoError(t),
		)

		return nil
	})

	time.Sleep(500 * time.Millisecond)

	treefmt(t,
		withArgs("-c", "--formatters", "echo"),
		withConfig(configPath, cfg),
		withNoError(t),
	)

	as.NoError(eg.Wait())
}

type options struct {
	args []string
	env  map[string]string

	config struct {
		path  string
		value *config.Config
	}

	assertStdout func([]byte)
	assertStderr func([]byte)

	assertError func(*require.Assertions, error)
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

func withEnv(env map[string]string) option {
	return func(o *options) {
		o.env = env
	}
}

func withConfig(path string, cfg *config.Config) option {
	return func(o *options) {
		o.config.path = path
		o.config.value = cfg
	}
}

func withConfigFunc(path string, fn func() *config.Config) option {
	return func(o *options) {
		o.config.path = path
		o.config.value = fn()
	}
}

func withStats(t *testing.T, expected map[stats.Type]int) option {
	t.Helper()

	return func(o *options) {
		o.assertStats = func(s *stats.Stats) {
			for k, v := range expected {
				require.Equal(t, v, s.Value(k), k.String())
			}
		}
	}
}

func withError(fn func(*require.Assertions, error)) option {
	return func(o *options) {
		o.assertError = fn
	}
}

func withNoError(t *testing.T) option {
	t.Helper()

	return func(o *options) {
		o.assertError = func(as *require.Assertions, err error) {
			as.NoError(err)
		}
	}
}

func withStdout(fn func([]byte)) option {
	return func(o *options) {
		o.assertStdout = fn
	}
}

func withStderr(fn func([]byte)) option {
	return func(o *options) {
		o.assertStderr = fn
	}
}

//nolint:unparam
func withModtimeBump(path string, bump time.Duration) option {
	return func(o *options) {
		o.bump.path = path
		o.bump.modtime = bump
	}
}

func treefmt(
	t *testing.T,
	opt ...option,
) {
	t.Helper()

	as := require.New(t)

	// build options
	opts := &options{}
	for _, option := range opt {
		option(opts)
	}

	// set env
	for k, v := range opts.env {
		t.Logf("setting env %s=%s", k, v)
		t.Setenv(k, v)
	}

	defer func() {
		// unset env variables after executing
		for k := range opts.env {
			t.Setenv(k, "")
		}
	}()

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

	tempStdout := test.TempFile(t, tempDir, "stdout", nil)
	tempStderr := test.TempFile(t, tempDir, "stderr", nil)

	// capture standard outputs before swapping them
	stdout := os.Stdout
	stderr := os.Stderr

	// swap them temporarily
	os.Stdout = tempStdout
	os.Stderr = tempStderr

	log.SetOutput(tempStdout)

	defer func() {
		// swap outputs back
		os.Stdout = stdout
		os.Stderr = stderr
		log.SetOutput(stderr)
	}()

	// run the command
	root, statz := cmd.NewRoot()

	root.SetArgs(args)
	root.SetOut(tempStdout)
	root.SetErr(tempStderr)

	// execute the command
	cmdErr := root.Execute()

	// reset and read the temporary outputs
	if _, resetErr := tempStdout.Seek(0, 0); resetErr != nil {
		t.Fatal(fmt.Errorf("failed to reset temp output for reading: %w", resetErr))
	}

	if _, resetErr := tempStderr.Seek(0, 0); resetErr != nil {
		t.Fatal(fmt.Errorf("failed to reset temp output for reading: %w", resetErr))
	}

	// read back stderr and validate
	out, readErr := io.ReadAll(tempStderr)
	if readErr != nil {
		t.Fatal(fmt.Errorf("failed to read temp stderr: %w", readErr))
	}

	if opts.assertStderr != nil {
		opts.assertStderr(out)
	}

	t.Log("\n" + string(out))

	// read back stdout and validate
	out, readErr = io.ReadAll(tempStdout)
	if readErr != nil {
		t.Fatal(fmt.Errorf("failed to read temp stdout: %w", readErr))
	}

	t.Log("\n" + string(out))

	if opts.assertStdout != nil {
		opts.assertStdout(out)
	}

	// assert other properties

	if opts.assertStats != nil {
		opts.assertStats(statz)
	}

	if opts.assertError != nil {
		opts.assertError(as, cmdErr)
	}
}
