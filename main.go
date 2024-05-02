package main

import (
	_ "embed"
	"fmt"
	"os"

	"git.numtide.com/numtide/treefmt/build"
	"git.numtide.com/numtide/treefmt/cli"
	"github.com/alecthomas/kong"
)

// We embed the sample toml file for use with the init flag.
//
//go:embed init.toml
var initBytes []byte

func main() {
	// This is to maintain compatibility with 1.0.0 which allows specifying the version with a `treefmt --version` flag
	// on the 'default' command. With Kong it would be better to have `treefmt version` so it would be treated as a
	// separate command. As it is, we would need to weaken some of the `existingdir` and `existingfile` checks kong is
	// doing for us in the default format command.
	for _, arg := range os.Args {
		if arg == "--version" || arg == "-V" {
			fmt.Printf("%s %s\n", build.Name, build.Version)
			return
		} else if arg == "--init" || arg == "-i" {
			if err := os.WriteFile("treefmt.toml", initBytes, 0o644); err != nil {
				fmt.Printf("Failed to write treefmt.toml: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Generated treefmt.toml. Now it's your turn to edit it.\n")
			return
		}
	}

	ctx := kong.Parse(&cli.Cli)
	cli.ConfigureLogging()
	ctx.FatalIfErrorf(ctx.Run())
}
