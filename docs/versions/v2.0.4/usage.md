---
outline: deep
---

# Usage

`treefmt` has the following specification:

```
Usage: treefmt [<paths> ...] [flags]

Arguments:
  [<paths> ...]    Paths to format. Defaults to formatting the whole tree.

Flags:
  -h, --help                         Show context-sensitive help.
      --allow-missing-formatter      Do not exit with error if a configured formatter is missing.
  -C, --working-directory="."        Run as if treefmt was started in the specified working directory instead of the current working directory.
      --no-cache                     Ignore the evaluation cache entirely. Useful for CI.
  -c, --clear-cache                  Reset the evaluation cache. Use in case the cache is not precise enough.
      --config-file=STRING           Load the config file from the given path (defaults to searching upwards for treefmt.toml).
      --fail-on-change               Exit with error if any changes were made. Useful for CI.
  -f, --formatters=FORMATTERS,...    Specify formatters to apply. Defaults to all formatters.
      --tree-root=STRING             The root directory from which treefmt will start walking the filesystem (defaults to the directory containing the config file) ($PRJ_ROOT).
      --tree-root-file=STRING        File to search for to find the project root (if --tree-root is not passed).
      --walk="auto"                  The method used to traverse the files within --tree-root. Currently supports 'auto', 'git' or 'filesystem'.
  -v, --verbose                      Set the verbosity of logs e.g. -vv ($LOG_LEVEL).
  -V, --version                      Print version.
  -i, --init                         Create a new treefmt.toml.
  -u, --on-unmatched=warn            Log paths that did not match any formatters at the specified log level, with fatal exiting the process with an error. Possible values are
                                     <debug|info|warn|error|fatal>.
      --stdin                        Format the context passed in via stdin.
      --cpu-profile=STRING           The file into which a cpu profile will be written.
      --ci                           Runs treefmt in a CI mode, enabling --no-cache, --fail-on-change and adjusting some other settings best suited to a CI use case.
```

## Arguments

### `[<paths> ...]`

Paths to format. Defaults to formatting the whole tree

## Flags

### `-h, --help`

Prints available flags and options

### `--allow-missing-formatter`

Do not exit with an error if some of the configured formatters are missing.

### `-C, --working-directory="."`

Run as if `treefmt` was started in the specified working directory instead of the current working directory

### `--no-cache`

Tells `treefmt` to ignore the evaluation cache entirely.

With this flag, you can avoid cache invalidation issues, if any. Typically, the machine that is running `treefmt` in
CI is starting with a fresh environment each time, so any calculated cache is lost.

The `--no-cache` flag eliminates unnecessary work in CI.

### `--config-file <config-file>`

Run with the specified config file.

### `--fail-on-change`

Exit with error if any changes were made.

This is useful for CI if you want to detect if someone forgot to format their code.

### `-f, --formatters <formatters>...`

Specify formatters to apply. Defaults to all formatters.

### `--tree-root="."`

The root directory from which `treefmt` will start walking the filesystem.

### `--walk <auto|git|filesystem>`

The method used to traverse the files within `--tree-root`. Currently supports `auto`, `git` or `filesystem`.

Default is `auto`, where we will detect if the `<tree-root>` is a git repository and use the `git` walker for
traversal. If not we will fall back to the `filesystem` walker.

### `-v, --verbose`

Set the verbosity of logs e.g. `-vv`. Can also be set with an integer value in an env variable `$LOG_LEVEL`.

Log verbosity is based off the number of 'v' used. With one `-v`, your logs will display `[INFO]` and `[ERROR]` messages,
while `-vv` will also show `[DEBUG]` messages.

### `--init`

Create a new `treefmt.toml`.

### `-u --on-unmatched`

Log paths that did not match any formatters at the specified log level, with fatal exiting the process with an error. Possible values are <debug|info|warn|error|fatal>.

[default: warn]

### `--stdin`

Format the context passed in via stdin.

### `--cpu-profile`

The file into which a cpu profile will be written.

### `--ci`

Runs treefmt in a CI mode which does the following:

-   ensures `INFO` level logging at a minimum
-   enables `--no-cache` and `--fail-on-change`
-   introduces a small startup delay so we do not start processing until the second after the process started, thereby
    ensuring the accuracy of our change detection based on second-level `modtime`.

### `-V, --version`

Print version.

## CI integration

Typically, you would use `treefmt` in CI with the `--ci` flag.

You can configure a `treefmt` job in a GitHub pipeline for Ubuntu with `nix-shell` like this:

```yaml
name: treefmt
on:
    pull_request:
    push:
        branches: main
jobs:
    formatter:
        runs-on: ubuntu-latest
        steps:
            - uses: actions/checkout@v4
            - uses: cachix/install-nix-action@v26
              with:
                  nix_path: nixpkgs=channel:nixos-unstable
            - uses: cachix/cachix-action@v14
              with:
                  name: nix-community
                  authToken: "${{ secrets.CACHIX_AUTH_TOKEN }}"
            - name: treefmt
              run: nix-shell -p treefmt --run "treefmt --ci"
```
