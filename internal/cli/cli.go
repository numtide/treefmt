package cli

import (
	"github.com/charmbracelet/log"
)

var Cli = Options{}

type Options struct {
	AllowMissingFormatter bool     `default:"false" help:"Do not exit with error if a configured formatter is missing"`
	ClearCache            bool     `short:"c" help:"Reset the evaluation cache. Use in case the cache is not precise enough"`
	ConfigFile            string   `type:"existingfile" default:"./treefmt.toml"`
	Formatters            []string `help:"Specify formatters to apply. Defaults to all formatters"`
	TreeRoot              string   `type:"existingdir" default:"."`
	Verbosity             int      `name:"verbose" short:"v" type:"counter" default:"0" env:"LOG_LEVEL" help:"Set the verbosity of logs e.g. -vv"`

	Format Format `cmd:"" default:"."`
}

func (c *Options) Configure() {
	log.SetReportTimestamp(false)

	if c.Verbosity == 0 {
		log.SetLevel(log.WarnLevel)
	} else if c.Verbosity == 1 {
		log.SetLevel(log.InfoLevel)
	} else if c.Verbosity >= 2 {
		log.SetLevel(log.DebugLevel)
	}
}
