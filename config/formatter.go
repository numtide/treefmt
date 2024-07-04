package config

type Formatter struct {
	// Command is the command to invoke when applying this Formatter.
	Command string `toml:"command"`
	// Options are an optional list of args to be passed to Command.
	Options []string `toml:"options,omitempty"`
	// Includes is a list of glob patterns used to determine whether this Formatter should be applied against a path.
	Includes []string `toml:"includes,omitempty"`
	// Excludes is an optional list of glob patterns used to exclude certain files from this Formatter.
	Excludes []string `toml:"excludes,omitempty"`
	// Indicates the order of precedence when executing this Formatter in a sequence of Formatters.
	Priority int `toml:"priority,omitempty"`
	// BatchSize controls the maximum number of paths to apply to the formatter in one invocation.
	BatchSize int `toml:"batch_size"`
}
