package format

import "github.com/BurntSushi/toml"

type Config struct {
	Formatters map[string]*Formatter `toml:"formatter"`
}

func ReadConfigFile(path string) (cfg *Config, err error) {
	_, err = toml.DecodeFile(path, &cfg)
	return
}
