# Usage

You can run treefmt by executing:

`$ treefmt`

or, if it's not in your `$PATH`:

`$ ./treefmt`

Treefmt has the following specification:

```
treefmt [FLAGS] [OPTIONS] [--] [paths]...
```

## Flags

`--allow-missing-formatter`

> Do not exit with an error if some of the configured formatters are missing.

`--clear-cache`

> Reset the evaluation cache. Invalidation should happen automatically if the formatting tool has been updated, or if the files to format have changed. If cache wasn't properly invalidated, you can use this flag to clear the cache.

`--fail-on-change`

> Exit with error if some files require re-formatting. This is useful for your CI if you want to detect if the contributed code was forgotten to be formatted.

`-h, --help`

> Prints available flags and options

`--init`

> Creates a new config file `treefmt.toml`.

`--no-cache`

> Tells `treefmt` to ignore the evaluation cache entirely. With this flag, you can avoid cache invalidation issues, if any. Typically, the machine that is running treefmt in the CI is starting with a fresh environment each time, so any calculated cache is lost. The `--no-cache` flag eliminates unnecessary work in the CI.

`-q, --quiet`

> Don't print output to stderr.

`--stdin`

> Format the content passed in stdin.

`-V, --version`

> Print version information.

`-v, --verbose`

> Change the log verbosity. Log verbosity is based off the number of 'v' used. With one `-v`, your logs will display `[INFO]` and `[ERROR]` messages, while `-vv` will also show `[DEBUG]` messages.

`--config-file <config-file>`

> Run with the specified config file which is not in the project tree.

`-f, --formatters <formatters>...`

> Only apply selected formatters. Defaults to all formatters.

`--tree-root <tree-root>`

> Set the path to the tree root directory where treefmt will look for the files to format. Defaults to the folder holding the `treefmt.toml` file. It’s mostly useful in combination with —config-file to specify the project root which won’t coincide with the directory holding `treefmt.toml`.

`-C <work-dir>`

> Run as if `treefmt` was started in <work-dir> instead of the current working directory (default: .). Equivalent to cd `<work dir>; treefmt`.

## Arguments

`<paths>...`

> Paths to format. Defaults to formatting the whole tree
## CI integration
Typically, you would use treefmt in the CI with the `--fail-on-change` and `--no-cache flags`. Find the explanations above. 

You can you set a `treefmt` job in the GitHub pipeline for Ubuntu with nix-shell like this:

```
name: treefmt
on:
  pull_request:
  push:
    branches: master
jobs:
  formatter:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v1
    - uses: cachix/install-nix-action@v12
      with:
        nix_path: nixpkgs=channel:nixos-unstable
    - uses: cachix/cachix-action@v10
      with:
        name: nix-community
        authToken: '${{ secrets.CACHIX_AUTH_TOKEN }}'
    - name: treefmt
      run: nix-shell --run "treefmt --fail-on-change --no-cache"
```