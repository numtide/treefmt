package cli

import "github.com/charmbracelet/log"

var Cli struct {
	Log LogOptions `embed:""`

	ConfigFile string `type:"existingfile" default:"./treefmt.toml"`
	TreeRoot   string `type:"existingdir" default:"."`
	ClearCache bool   `short:"c" help:"Reset the evaluation cache. Use in case the cache is not precise enough"`

	Format Format `cmd:"" default:"."`
}

type LogOptions struct {
	Verbosity int `name:"verbose" short:"v" type:"counter" default:"0" env:"LOG_LEVEL" help:"Set the verbosity of logs e.g. -vv"`
}

func (lo *LogOptions) ConfigureLogger() {
	log.SetReportTimestamp(false)

	if lo.Verbosity == 0 {
		log.SetLevel(log.WarnLevel)
	} else if lo.Verbosity == 1 {
		log.SetLevel(log.InfoLevel)
	} else if lo.Verbosity >= 2 {
		log.SetLevel(log.DebugLevel)
	}
}
