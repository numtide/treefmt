package cli

import (
	"git.numtide.com/numtide/treefmt/walk"
	"github.com/alecthomas/kong"
	"github.com/charmbracelet/log"
)

var Cli = Format{}

type Format struct {
	AllowMissingFormatter bool               `default:"false" help:"Do not exit with error if a configured formatter is missing."`
	WorkingDirectory      kong.ChangeDirFlag `default:"." short:"C" help:"Run as if treefmt was started in the specified working directory instead of the current working directory."`
	NoCache               bool               `help:"Ignore the evaluation cache entirely. Useful for CI."`
	ClearCache            bool               `short:"c" help:"Reset the evaluation cache. Use in case the cache is not precise enough."`
	ConfigFile            string             `type:"existingfile" default:"./treefmt.toml" help:"The config file to use."`
	FailOnChange          bool               `help:"Exit with error if any changes were made. Useful for CI."`
	Formatters            []string           `short:"f" help:"Specify formatters to apply. Defaults to all formatters."`
	TreeRoot              string             `type:"existingdir" default:"." help:"The root directory from which treefmt will start walking the filesystem."`
	Walk                  walk.Type          `enum:"auto,git,filesystem" default:"auto" help:"The method used to traverse the files within --tree-root. Currently supports 'auto', 'git' or 'filesystem'."`
	Verbosity             int                `name:"verbose" short:"v" type:"counter" default:"0" env:"LOG_LEVEL" help:"Set the verbosity of logs e.g. -vv."`
	Version               bool               `name:"version" short:"V" help:"Print version."`
	Init                  bool               `name:"init" short:"i" help:"Create a new treefmt.toml."`

	Paths []string `name:"paths" arg:"" type:"path" optional:"" help:"Paths to format. Defaults to formatting the whole tree."`
	Stdin bool     `help:"Format the context passed in via stdin."`

	CpuProfile string `optional:"" help:"The file into which a cpu profile will be written."`
}

func ConfigureLogging() {
	log.SetReportTimestamp(false)

	if Cli.Verbosity == 0 {
		log.SetLevel(log.WarnLevel)
	} else if Cli.Verbosity == 1 {
		log.SetLevel(log.InfoLevel)
	} else if Cli.Verbosity > 1 {
		log.SetLevel(log.DebugLevel)
	}
}
