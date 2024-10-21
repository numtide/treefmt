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
	cp "github.com/otiai10/copy"
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
		treefmt(t, withNoError(t), withOutput(checkOutput(log.WarnLevel)))
	})

	// should exit with error when using fatal
	t.Run("fatal", func(t *testing.T) {
		errorFn := func(err error) {
			as.ErrorContains(err, fmt.Sprintf("no formatter for path: %s", paths[0]))
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
				withOutput(checkOutput(level)),
			)

			t.Setenv("TREEFMT_ON_UNMATCHED", levelStr)

			treefmt(t,
				withArgs("-vv"),
				withNoError(t),
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

		treefmt(t,
			withArgs("--on-unmatched", "foo"),
			withError(errorFn("foo")),
		)

		t.Setenv("TREEFMT_ON_UNMATCHED", "bar")

		treefmt(t, withError(errorFn("bar")))
	})
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
		treefmt(t,
			withError(func(err error) {
				as.ErrorIs(err, format.ErrCommandNotFound)
			}),
		)
	})

	t.Run("arg", func(t *testing.T) {
		treefmt(t,
			withArgs("--allow-missing-formatter"),
			withNoError(t),
		)
	})

	t.Run("env", func(t *testing.T) {
		t.Setenv("TREEFMT_ALLOW_MISSING_FORMATTER", "true")
		treefmt(t, withNoError(t))
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
		treefmt(t,
			withNoError(t),
			withModtimeBump(tempDir, time.Second),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 32,
				stats.Matched:   3,
				stats.Formatted: 3,
				stats.Changed:   3,
			}),
		)
	})

	t.Run("args", func(t *testing.T) {
		treefmt(t,
			withArgs("--formatters", "elm,nix"),
			withModtimeBump(tempDir, time.Second),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 32,
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
				stats.Traversed: 32,
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
				stats.Traversed: 32,
				stats.Matched:   1,
				stats.Formatted: 1,
				stats.Changed:   1,
			}),
		)

		// bad name
		treefmt(t,
			withArgs("--formatters", "foo"),
			withError(func(err error) {
				as.Errorf(err, "formatter not found in config: foo")
			}),
		)
	})

	t.Run("env", func(t *testing.T) {
		t.Setenv("TREEFMT_FORMATTERS", "ruby,nix")

		treefmt(t,
			withNoError(t),
			withModtimeBump(tempDir, time.Second),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 32,
				stats.Matched:   2,
				stats.Formatted: 2,
				stats.Changed:   2,
			}),
		)

		t.Setenv("TREEFMT_FORMATTERS", "bar,foo")

		treefmt(t,
			withError(func(err error) {
				as.Errorf(err, "formatter not found in config: bar")
			}),
		)
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
			stats.Traversed: 32,
			stats.Matched:   32,
			stats.Formatted: 32,
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
			stats.Traversed: 32,
			stats.Matched:   31,
			stats.Formatted: 31,
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
			stats.Traversed: 32,
			stats.Matched:   25,
			stats.Formatted: 25,
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
			stats.Traversed: 32,
			stats.Matched:   23,
			stats.Formatted: 23,
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
			stats.Traversed: 32,
			stats.Matched:   22,
			stats.Formatted: 22,
			stats.Changed:   0,
		}),
	)

	t.Setenv("TREEFMT_FORMATTER_ECHO_EXCLUDES", "") // reset

	// adjust the includes for echo to only include elm files
	echo.Includes = []string{"*.elm"}

	treefmt(t,
		withArgs("-c"),
		withConfig(configPath, cfg),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 32,
			stats.Matched:   1,
			stats.Formatted: 1,
			stats.Changed:   0,
		}),
	)

	// add js files to echo formatter via env
	t.Setenv("TREEFMT_FORMATTER_ECHO_INCLUDES", "*.elm,*.js")

	treefmt(t,
		withArgs("-c"),
		withConfig(configPath, cfg),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 32,
			stats.Matched:   2,
			stats.Formatted: 2,
			stats.Changed:   0,
		}),
	)
}

func TestPrjRootEnvVariable(t *testing.T) {
	tempDir := test.TempExamples(t)
	configPath := filepath.Join(tempDir, "treefmt.toml")

	t.Setenv("PRJ_ROOT", tempDir)

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
	treefmt(t,
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 32,
			stats.Matched:   32,
			stats.Formatted: 32,
			stats.Changed:   32,
		}),
	)

	// cached run with no changes to underlying files
	treefmt(t,
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 32,
			stats.Matched:   32,
			stats.Formatted: 0,
			stats.Changed:   0,
		}),
	)

	// clear cache
	treefmt(t,
		withArgs("-c"),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 32,
			stats.Matched:   32,
			stats.Formatted: 32,
			stats.Changed:   32,
		}),
	)

	// cached run with no changes to underlying files
	treefmt(t,
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 32,
			stats.Matched:   32,
			stats.Formatted: 0,
			stats.Changed:   0,
		}),
	)

	// bump underlying files
	treefmt(t,
		withNoError(t),
		withModtimeBump(tempDir, time.Second),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 32,
			stats.Matched:   32,
			stats.Formatted: 32,
			stats.Changed:   32,
		}),
	)

	// no cache
	treefmt(t,
		withArgs("--no-cache"),
		withNoError(t),
		withStats(t, map[stats.Type]int{
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

	treefmt(t,
		withError(func(err error) {
			as.ErrorIs(err, format.ErrFormattingFailures)
		}),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 32,
			stats.Matched:   6,
			stats.Formatted: 0,
			stats.Changed:   0,
		}),
	)

	// running again should provide the same result
	treefmt(t,
		withError(func(err error) {
			as.ErrorIs(err, format.ErrFormattingFailures)
		}),
		withStats(t, map[stats.Type]int{
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
	treefmt(t,
		withNoError(t),
		withStats(t, map[stats.Type]int{
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

		treefmt(t,
			withConfig(configPath, cfg),
			withError(func(err error) {
				as.ErrorContains(err, "failed to find treefmt config file")
			}),
		)

		// now change to the examples temp directory
		as.NoError(os.Chdir(tempDir), "failed to change to temp directory")

		treefmt(t,
			withConfig(configPath, cfg),
			withNoError(t),
			withStats(t, map[stats.Type]int{
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

			treefmt(t,
				withArgs(args...),
				withConfig(configPath, cfg),
				withNoError(t),
				withStats(t, map[stats.Type]int{
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

		test.ChangeWorkDir(t, tempDir)

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

		// running with a cold cache, we should see the elm files being formatted, resulting in changes, which should
		// trigger an error
		treefmt(t,
			withArgs("--fail-on-change"),
			withConfig(configPath, cfg),
			withError(func(err error) {
				as.ErrorIs(err, formatCmd.ErrFailOnChange)
			}),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 32,
				stats.Matched:   2,
				stats.Formatted: 2,
				stats.Changed:   2,
			}),
		)

		// running with a hot cache, we should see matches for the elm files, but no attempt to format them as the
		// underlying files have not changed since we last ran
		treefmt(t,
			withArgs("--fail-on-change"),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 32,
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
			withError(func(err error) {
				as.ErrorIs(err, formatCmd.ErrFailOnChange)
			}),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 32,
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
				stats.Traversed: 32,
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
				stats.Traversed: 32,
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
				stats.Traversed: 32,
				stats.Matched:   8,
				stats.Formatted: 6,
				stats.Changed:   6,
			}))

		// run again, nothing should be formatted
		treefmt(t,
			withConfig(configPath, cfg),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 32,
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
				stats.Traversed: 32,
				stats.Matched:   8,
				stats.Formatted: 6,
				stats.Changed:   0, // echo doesn't affect the files so no changes expected
			}))

		// run again, nothing should be formatted
		treefmt(t,
			withConfig(configPath, cfg),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 32,
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
				stats.Traversed: 32,
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
				stats.Traversed: 32,
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
				"elm": {
					Command:  "test-fmt-append",
					Options:  []string{"   "},
					Includes: []string{"*.elm"},
				},
			},
		}

		// initial run
		treefmt(t,
			withConfig(configPath, cfg),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 33,
				stats.Matched:   3,
				stats.Formatted: 3,
				stats.Changed:   1,
			}))

		// tweak mod time of elm formatter
		newTime := time.Now().Add(-time.Minute)
		as.NoError(os.Chtimes(scriptPath, newTime, newTime))

		// cache entries for elm files should be invalidated
		treefmt(t,
			withConfig(configPath, cfg),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 33,
				stats.Matched:   3,
				stats.Formatted: 1,
				stats.Changed:   1,
			}))

		// running again with a hot cache, we should see nothing be formatted
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

		// tweak the size of elm formatter
		formatter, err := os.OpenFile(scriptPath, os.O_WRONLY|os.O_APPEND, 0o755)
		as.NoError(err, "failed to open elm formatter")

		_, err = formatter.Write([]byte(" ")) // add some whitespace
		as.NoError(err, "failed to append to elm formatter")
		as.NoError(formatter.Close(), "failed to close elm formatter")

		// cache entries for elm files should be invalidated
		treefmt(t,
			withConfig(configPath, cfg),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 33,
				stats.Matched:   3,
				stats.Formatted: 1,
				stats.Changed:   1,
			}))

		// running again with a hot cache, we should see nothing be formatted
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
				stats.Traversed: 32,
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
				stats.Traversed: 32,
				stats.Matched:   2,
				stats.Formatted: 0,
				stats.Changed:   0,
			}),
		)

		// add a formatter
		cfg.FormatterConfigs["elm"] = &config.Formatter{
			Command:  "test-fmt-append",
			Options:  []string{"   "},
			Includes: []string{"*.elm"},
		}

		// only the elm files should be formatted
		treefmt(t,
			withConfig(configPath, cfg),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 32,
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
				stats.Traversed: 32,
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
				stats.Traversed: 32,
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
				stats.Traversed: 32,
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
				stats.Traversed: 32,
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
				stats.Traversed: 32,
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
				stats.Traversed: 32,
				stats.Matched:   3,
				stats.Formatted: 0,
				stats.Changed:   0,
			}),
		)

		// remove the elm formatter
		delete(cfg.FormatterConfigs, "elm")

		// only python files should match, but no formatting should occur as not formatting signatures have been
		// affected
		treefmt(t,
			withConfig(configPath, cfg),
			withNoError(t),
			withStats(t, map[stats.Type]int{
				stats.Traversed: 32,
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
	treefmt(t,
		withConfig(configPath, cfg),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 0,
		}),
	)

	// add everything to the index
	gitCmd = exec.Command("git", "add", ".")
	as.NoError(gitCmd.Run(), "failed to add everything to the index")

	treefmt(t,
		withConfig(configPath, cfg),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 32,
			stats.Matched:   32,
			stats.Formatted: 32,
			stats.Changed:   0,
		}),
	)

	// remove python directory from the index
	gitCmd = exec.Command("git", "rm", "--cached", "python/*")
	as.NoError(gitCmd.Run(), "failed to remove python directory from the index")

	// we should traverse and match against fewer files, but no formatting should occur as no formatting signatures
	// are impacted
	treefmt(t,
		withConfig(configPath, cfg),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 29,
			stats.Matched:   29,
			stats.Formatted: 0,
			stats.Changed:   0,
		}),
	)

	// remove nixpkgs.toml from the filesystem but leave it in the index
	as.NoError(os.Remove(filepath.Join(tempDir, "nixpkgs.toml")))

	// walk with filesystem instead of with git
	// the .git folder contains 49 additional files
	// when added to the 31 we started with (32 minus nixpkgs.toml which we removed from the filesystem), we should
	// traverse 80 files.
	treefmt(t,
		withArgs("--walk", "filesystem"),
		withConfig(configPath, cfg),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 80,
			stats.Matched:   80,
			stats.Formatted: 49, // the echo formatter should only be applied to the new files
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
		withError(func(err error) {
			as.ErrorContains(err, "path foo not found")
		}),
	)

	// try with a path not in the git index, e.g. it is skipped
	_, err := os.Create(filepath.Join(tempDir, "foo.txt"))
	as.NoError(err)

	treefmt(t,
		withArgs("haskell", "foo.txt"),
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

func TestPathsArg(t *testing.T) {
	as := require.New(t)

	// capture current cwd, so we can replace it after the test is finished
	cwd, err := os.Getwd()
	as.NoError(err)

	t.Cleanup(func() {
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
			stats.Traversed: 32,
			stats.Matched:   32,
			stats.Formatted: 32,
			stats.Changed:   0,
		}),
	)

	// specify some explicit paths
	treefmt(t,
		withArgs("elm/elm.json", "haskell/Nested/Foo.hs"),
		withNoError(t),
		withStats(t, map[stats.Type]int{
			stats.Traversed: 2,
			stats.Matched:   2,
			stats.Formatted: 0,
			stats.Changed:   0,
		}),
	)

	// specify an absolute path
	absoluteInternalPath, err := filepath.Abs("elm/elm.json")
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
		withArgs("elm/elm.json", "haskell/Nested/Bar.hs"),
		withError(func(err error) {
			as.Errorf(err, "path haskell/Nested/Bar.hs not found")
		}),
	)

	// specify an absolute path outside the tree root
	absoluteExternalPath, err := filepath.Abs(externalFile.Name())
	as.NoError(err)
	as.FileExists(absoluteExternalPath, "external file must exist")

	treefmt(t,
		withArgs(absoluteExternalPath),
		withError(func(err error) {
			as.Errorf(err, "path %s not found within the tree root", absoluteExternalPath)
		}),
	)

	// specify a relative path outside the tree root
	relativeExternalPath := "../outside_tree.go"
	as.FileExists(relativeExternalPath, "exernal file must exist")

	treefmt(t,
		withArgs(relativeExternalPath),
		withError(func(err error) {
			as.Errorf(err, "path %s not found within the tree root", relativeExternalPath)
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
		withError(func(err error) {
			as.EqualError(err, "exactly one path should be specified when using the --stdin flag")
		}),
		withOutput(func(out []byte) {
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
		withOutput(func(out []byte) {
			as.Equal(`{ ...}: "hello"
`, string(out))
		}),
	)

	// the nix formatters should have reduced the example to the following

	// try a file that's outside of the project root
	os.Stdin = test.TempFile(t, "", "stdin", &contents)

	treefmt(t,
		withArgs("--stdin", "../test.nix"),
		withError(func(err error) {
			as.Errorf(err, "path ../test.nix not inside the tree root %s", tempDir)
		}),
		withOutput(func(out []byte) {
			as.Contains(string(out), "Error: path ../test.nix not inside the tree root")
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
		withOutput(func(out []byte) {
			as.Equal(`| col1   | col2      |
| ------ | --------- |
| nice   | fits      |
| oh no! | it's ugly |
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
			treefmt(t,
				withNoError(t),
				withStats(t, map[stats.Type]int{
					stats.Traversed: 32,
					stats.Matched:   32,
					stats.Formatted: 32,
					stats.Changed:   0,
				}),
			)

			// specify some explicit paths, relative to the tree root
			// this should not work, as we're in a subdirectory
			treefmt(t,
				withArgs("-c", "elm/elm.json", "haskell/Nested/Foo.hs"),
				withError(func(err error) {
					as.ErrorContains(err, "path elm/elm.json not found")
				}),
			)

			// specify some explicit paths, relative to the current directory
			treefmt(t,
				withArgs("-c", "elm.json", "../haskell/Nested/Foo.hs"),
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

func withConfigFunc(path string, fn func() *config.Config) option {
	return func(o *options) {
		o.config.path = path
		o.config.value = fn()
	}
}

func withStats(t *testing.T, expected map[stats.Type]int) option {
	return func(o *options) {
		o.assertStats = func(s *stats.Stats) {
			for k, v := range expected {
				require.Equal(t, v, s.Value(k), k.String())
			}
		}
	}
}

func withError(fn func(error)) option {
	return func(o *options) {
		o.assertError = fn
	}
}

func withNoError(t *testing.T) option {
	return func(o *options) {
		o.assertError = func(err error) {
			require.NoError(t, err)
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

func treefmt(
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
