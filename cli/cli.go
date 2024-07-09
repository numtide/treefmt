package cli

import (
	"os"

	"github.com/gobwas/glob"

	"git.numtide.com/numtide/treefmt/format"
	"git.numtide.com/numtide/treefmt/walker"
	"github.com/alecthomas/kong"
	"github.com/charmbracelet/log"
)

func New() *Format {
	return &Format{}
}

type Format struct {
	AllowMissingFormatter bool               `default:"false" help:"Do not exit with error if a configured formatter is missing."`
	WorkingDirectory      kong.ChangeDirFlag `default:"." short:"C" help:"Run as if treefmt was started in the specified working directory instead of the current working directory."`
	NoCache               bool               `help:"Ignore the evaluation cache entirely. Useful for CI."`
	ClearCache            bool               `short:"c" help:"Reset the evaluation cache. Use in case the cache is not precise enough."`
	ConfigFile            string             `type:"existingfile" help:"Load the config file from the given path (defaults to searching upwards for treefmt.toml)."`
	FailOnChange          bool               `help:"Exit with error if any changes were made. Useful for CI."`
	Formatters            []string           `short:"f" help:"Specify formatters to apply. Defaults to all formatters."`
	TreeRoot              string             `type:"existingdir" xor:"tree-root" help:"The root directory from which treefmt will start walking the filesystem (defaults to the directory containing the config file)."`
	TreeRootFile          string             `type:"string" xor:"tree-root" help:"File to search for to find the project root (if --tree-root is not passed)."`
	Walk                  walker.Type        `enum:"auto,git,filesystem" default:"auto" help:"The method used to traverse the files within --tree-root. Currently supports 'auto', 'git' or 'filesystem'."`

	Verbosity int  `name:"verbose" short:"v" type:"counter" default:"0" env:"LOG_LEVEL" help:"Set the verbosity of logs e.g. -vv."`
	Version   bool `name:"version" short:"V" help:"Print version."`
	Init      bool `name:"init" short:"i" help:"Create a new treefmt.toml."`

	OnUnmatched log.Level `name:"on-unmatched" short:"u" default:"warn" help:"Log paths that did not match any formatters at the specified log level, with fatal exiting the process with an error. Possible values are <debug|info|warn|error|fatal>."`

	Paths []string `name:"paths" arg:"" type:"path" optional:"" help:"Paths to format. Defaults to formatting the whole tree."`
	Stdin bool     `help:"Format the context passed in via stdin."`

	CpuProfile string `optional:"" help:"The file into which a cpu profile will be written."`

	formatters     map[string]*format.Formatter
	globalExcludes []glob.Glob

	fileCh      chan *walker.File
	formattedCh chan *walker.File
	processedCh chan *walker.File
}

func (f *Format) configureLogging() {
	log.SetReportTimestamp(false)
	log.SetOutput(os.Stderr)

	if f.Verbosity == 0 {
		log.SetLevel(log.WarnLevel)
	} else if f.Verbosity == 1 {
		log.SetLevel(log.InfoLevel)
	} else if f.Verbosity > 1 {
		log.SetLevel(log.DebugLevel)
	}
}
