package main

import (
	"github.com/alecthomas/kong"
	"github.com/numtide/treefmt/internal/cli"
)

func main() {
	ctx := kong.Parse(&cli.Cli)
	ctx.FatalIfErrorf(ctx.Run())
}
