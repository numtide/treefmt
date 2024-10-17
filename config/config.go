package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/numtide/treefmt/walk"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var ErrInvalidBatchSize = fmt.Errorf("batch size must be between 1 and 10,240")

// Config is used to represent the list of configured Formatters.
type Config struct {
	AllowMissingFormatter bool     `mapstructure:"allow-missing-formatter" toml:"allow-missing-formatter,omitempty"`
	BatchSize             int      `mapstructure:"batch-size" toml:"batch-size,omitempty"`
	CI                    bool     `mapstructure:"ci" toml:"ci,omitempty"`
	ClearCache            bool     `mapstructure:"clear-cache" toml:"-"` // not allowed in config
	CPUProfile            string   `mapstructure:"cpu-profile" toml:"cpu-profile,omitempty"`
	Excludes              []string `mapstructure:"excludes" toml:"excludes,omitempty"`
	FailOnChange          bool     `mapstructure:"fail-on-change" toml:"fail-on-change,omitempty"`
	Formatters            []string `mapstructure:"formatters" toml:"formatters,omitempty"`
	NoCache               bool     `mapstructure:"no-cache" toml:"-"` // not allowed in config
	OnUnmatched           string   `mapstructure:"on-unmatched" toml:"on-unmatched,omitempty"`
	TreeRoot              string   `mapstructure:"tree-root" toml:"tree-root,omitempty"`
	TreeRootFile          string   `mapstructure:"tree-root-file" toml:"tree-root-file,omitempty"`
	Verbose               uint8    `mapstructure:"verbose" toml:"verbose,omitempty"`
	Walk                  string   `mapstructure:"walk" toml:"walk,omitempty"`
	WorkingDirectory      string   `mapstructure:"working-dir" toml:"-"`
	Stdin                 bool     `mapstructure:"stdin" toml:"-"` // not allowed in config

	FormatterConfigs map[string]*Formatter `mapstructure:"formatter" toml:"formatter,omitempty"`

	Global struct {
		// Deprecated: Use Excludes
		Excludes []string `mapstructure:"excludes" toml:"excludes,omitempty"`
	} `mapstructure:"global" toml:"global,omitempty"`
}

type Formatter struct {
	// Command is the command to invoke when applying this Formatter.
	Command string `mapstructure:"command" toml:"command"`
	// Options are an optional list of args to be passed to Command.
	Options []string `mapstructure:"options,omitempty" toml:"options,omitempty"`
	// Includes is a list of glob patterns used to determine whether this Formatter should be applied against a path.
	Includes []string `mapstructure:"includes,omitempty" toml:"includes,omitempty"`
	// Excludes is an optional list of glob patterns used to exclude certain files from this Formatter.
	Excludes []string `mapstructure:"excludes,omitempty" toml:"excludes,omitempty"`
	// Indicates the order of precedence when executing this Formatter in a sequence of Formatters.
	Priority int `mapstructure:"priority,omitempty" toml:"priority,omitempty"`
}

// SetFlags appends our flags to the provided flag set.
// We have a flag matching most entries in Config, taking care to ensure the name matches the field name defined in the
// mapstructure tag.
// We rely on a flag's default value being provided in the event the same value was not specified in the config file.
func SetFlags(fs *pflag.FlagSet) {
	fs.Bool(
		"allow-missing-formatter", false,
		"Do not exit with error if a configured formatter is missing. (env $TREEFMT_ALLOW_MISSING_FORMATTER)",
	)
	fs.Uint("batch-size", 1024,
		"The maximum number of files to pass to a formatter at once. (env $TREEFMT_BATCH_SIZE)",
	)
	fs.Bool(
		"ci", false,
		"Runs treefmt in a CI mode, enabling --no-cache, --fail-on-change and adjusting some other settings "+
			"best suited to a CI use case. (env $TREEFMT_CI)",
	)
	fs.BoolP(
		"clear-cache", "c", false,
		"Reset the evaluation cache. Use in case the cache is not precise enough. (env $TREEFMT_CLEAR_CACHE)",
	)
	fs.String(
		"cpu-profile", "",
		"The file into which a cpu profile will be written. (env $TREEFMT_CPU_PROFILE)",
	)
	fs.StringSlice(
		"excludes", nil,
		"Exclude files or directories matching the specified globs. (env $TREEFMT_EXCLUDES)",
	)
	fs.Bool(
		"fail-on-change", false,
		"Exit with error if any changes were made. Useful for CI. (env $TREEFMT_FAIL_ON_CHANGE)",
	)
	fs.StringSliceP(
		"formatters", "f", nil,
		"Specify formatters to apply. Defaults to all configured formatters. (env $TREEFMT_FORMATTERS)",
	)
	fs.Bool(
		"no-cache", false,
		"Ignore the evaluation cache entirely. Useful for CI. (env $TREEFMT_NO_CACHE)",
	)
	fs.StringP(
		"on-unmatched", "u", "warn",
		"Log paths that did not match any formatters at the specified log level. Possible values are "+
			"<debug|info|warn|error|fatal>. (env $TREEFMT_ON_UNMATCHED)",
	)
	fs.Bool(
		"stdin", false,
		"Format the context passed in via stdin.",
	)
	fs.String(
		"tree-root", "",
		"The root directory from which treefmt will start walking the filesystem (defaults to the directory "+
			"containing the config file). (env $TREEFMT_TREE_ROOT)",
	)
	fs.String(
		"tree-root-file", "",
		"File to search for to find the tree root (if --tree-root is not passed). (env $TREEFMT_TREE_ROOT_FILE)",
	)
	fs.CountP(
		"verbose", "v",
		"Set the verbosity of logs e.g. -vv. (env $TREEFMT_VERBOSE)",
	)
	fs.String(
		"walk", "auto",
		"The method used to traverse the files within the tree root. Currently supports "+
			"<auto|git|filesystem>. (env $TREEFMT_WALK)",
	)
	fs.StringP(
		"working-dir", "C", ".",
		"Run as if treefmt was started in the specified working directory instead of the current working "+
			"directory. (env $TREEFMT_WORKING_DIR)",
	)
}

// NewViper creates a Viper instance pre-configured with the following options:
// * TOML config type
// * automatic env enabled
// * `TREEFMT_` env prefix for environment variables
// * replacement of `-` and `.` with `_` when mapping flags to env e.g. `global.excludes` => `TREEFMT_GLOBAL_EXCLUDES`.
func NewViper() (*viper.Viper, error) {
	v := viper.New()

	// Enforce toml (may open this up to other formats in the future)
	v.SetConfigType("toml")

	// Allow env overrides for config and flags.
	v.SetEnvPrefix("treefmt")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))

	// unset some env variables that we don't want automatically applied
	if err := os.Unsetenv("TREEFMT_STDIN"); err != nil {
		return nil, fmt.Errorf("failed to unset TREEFMT_STDIN: %w", err)
	}

	return v, nil
}

// FromViper takes a viper instance and produces a Config instance.
func FromViper(v *viper.Viper) (*Config, error) {
	configReset := map[string]any{
		"ci":          false,
		"clear-cache": false,
		"no-cache":    false,
		"stdin":       false,
		"working-dir": ".",
	}

	// reset certain values which are not allowed to be specified in the config file
	if err := v.MergeConfigMap(configReset); err != nil {
		return nil, fmt.Errorf("failed to overwrite config values: %w", err)
	}

	// read config from viper
	var err error

	cfg := &Config{}

	if err = v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// resolve the working directory to an absolute path
	cfg.WorkingDirectory, err = filepath.Abs(cfg.WorkingDirectory)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for working directory: %w", err)
	}

	// if the stdin flag was passed, we force the stdin walk type
	if cfg.Stdin {
		cfg.Walk = walk.Stdin.String()
	}

	// determine the tree root
	if cfg.TreeRoot == "" {
		// if none was specified, we first try with tree-root-file
		if cfg.TreeRootFile != "" {
			// search the tree root using the --tree-root-file if specified
			_, cfg.TreeRoot, err = FindUp(cfg.WorkingDirectory, cfg.TreeRootFile)
			if err != nil {
				return nil, fmt.Errorf("failed to find tree-root based on tree-root-file: %w", err)
			}
		} else {
			// otherwise fallback to the directory containing the config file
			cfg.TreeRoot = filepath.Dir(v.ConfigFileUsed())
		}
	}

	// resolve tree root to an absolute path
	if cfg.TreeRoot, err = filepath.Abs(cfg.TreeRoot); err != nil {
		return nil, fmt.Errorf("failed to get absolute path for tree root: %w", err)
	}

	// prefer top level excludes, falling back to global.excludes for backwards compatibility
	if len(cfg.Excludes) == 0 {
		cfg.Excludes = cfg.Global.Excludes
	}

	// filter formatters based on provided names
	if len(cfg.Formatters) > 0 {
		filtered := make(map[string]*Formatter)

		// check if the provided names exist in the config
		for _, name := range cfg.Formatters {
			formatterCfg, ok := cfg.FormatterConfigs[name]
			if !ok {
				return nil, fmt.Errorf("formatter %v not found in config", name)
			}

			filtered[name] = formatterCfg
		}

		// updated formatters
		cfg.FormatterConfigs = filtered
	}

	// ci mode
	if cfg.CI {
		cfg.NoCache = true
		cfg.FailOnChange = true

		// ensure at least info level logging
		if cfg.Verbose < 1 {
			cfg.Verbose = 1
		}
	}

	// validate batch size
	// todo what is a reasonable upper limit on this?

	// default if it isn't set (e.g. in tests when using Config directly)
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 1024
	}

	if !(1 <= cfg.BatchSize && cfg.BatchSize <= 10240) {
		return nil, ErrInvalidBatchSize
	}

	l := log.WithPrefix("config")
	l.Infof("batch size = %d", cfg.BatchSize)

	return cfg, nil
}

func FindUp(searchDir string, fileNames ...string) (path string, dir string, err error) {
	for _, dir := range eachDir(searchDir) {
		for _, f := range fileNames {
			path := filepath.Join(dir, f)
			if fileExists(path) {
				return path, dir, nil
			}
		}
	}

	return "", "", fmt.Errorf("could not find %s in %s", fileNames, searchDir)
}

func eachDir(path string) (paths []string) {
	path, err := filepath.Abs(path)
	if err != nil {
		return
	}

	paths = []string{path}

	if path == "/" {
		return
	}

	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == os.PathSeparator {
			path = path[:i]
			if path == "" {
				path = "/"
			}

			paths = append(paths, path)
		}
	}

	return
}

func fileExists(path string) bool {
	// Some broken filesystems like SSHFS return file information on stat() but
	// then cannot open the file. So we use os.Open.
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	// Next, check that the file is a regular file.
	fi, err := f.Stat()
	if err != nil {
		return false
	}

	return fi.Mode().IsRegular()
}
