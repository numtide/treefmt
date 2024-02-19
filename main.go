package main

import (
	"fmt"
	"os"

	"git.numtide.com/numtide/treefmt/build"
	"git.numtide.com/numtide/treefmt/cli"
	"github.com/alecthomas/kong"
)

func main() {
	// This is to maintain compatibility with 1.0.0 which allows specifying the version with a `treefmt --version` flag
	// on the 'default' command. With Kong it would be better to have `treefmt version` so it would be treated as a
	// separate command. As it is, we would need to weaken some of the `existingdir` and `existingfile` checks kong is
	// doing for us in the default format command.
	for _, arg := range os.Args {
		if arg == "--version" || arg == "-V" {
			fmt.Printf("%s %s\n", build.Name, build.Version)
			return
		}
	}

	ctx := kong.Parse(&cli.Cli)
	ctx.FatalIfErrorf(ctx.Run())
}
