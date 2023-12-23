package main

import (
	"git.numtide.com/numtide/treefmt/internal/cli"
	"github.com/alecthomas/kong"
)

func main() {
	ctx := kong.Parse(&cli.Cli)
	ctx.FatalIfErrorf(ctx.Run())
}
