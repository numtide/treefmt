package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/numtide/treefmt/walk"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config is used to represent the list of configured Formatters.
type Config struct {
	AllowMissingFormatter bool     `mapstructure:"allow-missing-formatter,omitempty"`
	CI                    bool     `mapstructure:"ci,omitempty"`
	ClearCache            bool     `mapstructure:"clear-cache,omitempty"`
	CPUProfile            string   `mapstructure:"cpu-profile,omitempty"`
	Excludes              []string `mapstructure:"excludes"`
	FailOnChange          bool     `mapstructure:"fail-on-change,omitempty"`
	Formatters            []string `mapstructure:"formatters,omitempty"`
	NoCache               bool     `mapstructure:"no-cache,omitempty"`
	OnUnmatched           string   `mapstructure:"on-unmatched,omitempty"`
	TreeRoot              string   `mapstructure:"tree-root,omitempty"`
	TreeRootFile          string   `mapstructure:"tree-root-file,omitempty"`
	Verbosity             uint8    `mapstructure:"verbose"`
	Walk                  string   `mapstructure:"walk,omitempty"`
	WorkingDirectory      string   `mapstructure:"working-dir,omitempty"`
	Stdin                 bool     `mapstructure:"stdin,omitempty"`

	FormatterConfigs map[string]*Formatter `mapstructure:"formatter"`

	Global struct {
		// Deprecated: Use Excludes
		Excludes []string `mapstructure:"excludes,omitempty"`
	} `mapstructure:"global"`
}

type Formatter struct {
	// Command is the command to invoke when applying this Formatter.
	Command string `mapstructure:"command"`
	// Options are an optional list of args to be passed to Command.
	Options []string `mapstructure:"options,omitempty"`
	// Includes is a list of glob patterns used to determine whether this Formatter should be applied against a path.
	Includes []string `mapstructure:"includes,omitempty"`
	// Excludes is an optional list of glob patterns used to exclude certain files from this Formatter.
	Excludes []string `mapstructure:"excludes,omitempty"`
	// Indicates the order of precedence when executing this Formatter in a sequence of Formatters.
	Priority int `mapstructure:"priority,omitempty"`
}

// SetFlags appends our flags to the provided flag set.
// We have a flag matching most entries in Config, taking care to ensure the name matches the field name defined in the
// mapstructure tag.
// We can rely on a flag's default value being provided in the event the same value was not specified in the config
// file.
func SetFlags(fs *pflag.FlagSet) *pflag.FlagSet {
	fs.Bool(
		"allow-missing-formatter", false,
		"Do not exit with error if a configured formatter is missing. (env $TREEFMT_ALLOW_MISSING_FORMATTER)",
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

	fs.StringSliceP(
		"excludes", "e", nil,
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

	fs.Bool("stdin", false, "Format the context passed in via stdin.")

	fs.String(
		"tree-root", "",
		"The root directory from which treefmt will start walking the "+
			"filesystem (defaults to the directory containing the config file). (env $TREEFMT_TREE_ROOT)",
	)

	fs.String(
		"tree-root-file", "",
		"File to search for to find the tree root (if --tree-root is not passed). (env $TREEFMT_TREE_ROOT_FILE)",
	)

	fs.String(
		"walk", "auto",
		"The method used to traverse the files within the tree root. Currently supports 'auto', 'git' or "+
			"'filesystem'. ($TREEFMT_WALK)",
	)

	fs.CountP("verbose", "v", "Set the verbosity of logs e.g. -vv. (env $TREEFMT_VERBOSE)")

	fs.StringP(
		"working-dir", "C", ".",
		"Run as if treefmt was started in the specified working directory instead of the current working "+
			"directory. $(TREEFMT_WORKING_DIR)",
	)

	return fs
}

// NewViper creates a Viper instance pre-configured with the following options:
// * TOML config type
// * automatic env enabled
// * `TREEFMT_` env prefix for environment variables
// * replacement of `-` and `.` with `_` when mapping from flags to env.
func NewViper() *viper.Viper {
	v := viper.New()

	// Enforce toml (may open this up to other formats in the future)
	v.SetConfigType("toml")

	// Allow env overrides for config and flags.
	v.SetEnvPrefix("treefmt")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))

	return v
}

// FromViper takes a viper instance and produces a Config instance.
func FromViper(v *viper.Viper) (*Config, error) {
	cfg := &Config{}

	var err error

	if err = v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// resolve the working directory to an absolute path
	cfg.WorkingDirectory, err = filepath.Abs(cfg.WorkingDirectory)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for working directory: %w", err)
	}

	// determine the tree root
	if cfg.TreeRoot == "" {
		// if none was specified, we first try with tree-root-file
		if cfg.TreeRootFile != "" {
			// search the tree root using the --tree-root-file if specified
			_, cfg.TreeRoot, err = walk.FindUp(cfg.WorkingDirectory, cfg.TreeRootFile)
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
		if cfg.Verbosity < 1 {
			cfg.Verbosity = 1
		}
	}

	return cfg, nil
}
