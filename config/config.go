package config

import "github.com/BurntSushi/toml"

// Config is used to represent the list of configured Formatters.
type Config struct {
	Global struct {
		// Excludes is an optional list of glob patterns used to exclude certain files from all formatters.
		Excludes []string
	}
	Formatters map[string]*Formatter `toml:"formatter"`
}

// ReadFile reads from path and unmarshals toml into a Config instance.
func ReadFile(path string) (cfg *Config, err error) {
	_, err = toml.DecodeFile(path, &cfg)
	return
}
