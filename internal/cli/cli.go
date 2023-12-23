package cli

import "github.com/charmbracelet/log"

var Cli = Options{}

type Options struct {
	Verbosity  int    `name:"verbose" short:"v" type:"counter" default:"0" env:"LOG_LEVEL" help:"Set the verbosity of logs e.g. -vv"`
	ConfigFile string `type:"existingfile" default:"./treefmt.toml"`
	TreeRoot   string `type:"existingdir" default:"."`
	ClearCache bool   `short:"c" help:"Reset the evaluation cache. Use in case the cache is not precise enough"`

	Format Format `cmd:"" default:"."`
}

func (c *Options) ConfigureLogger() {
	log.SetReportTimestamp(false)

	if c.Verbosity == 0 {
		log.SetLevel(log.WarnLevel)
	} else if c.Verbosity == 1 {
		log.SetLevel(log.InfoLevel)
	} else if c.Verbosity >= 2 {
		log.SetLevel(log.DebugLevel)
	}
}
