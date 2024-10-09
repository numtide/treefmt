---
outline: deep
---

# Usage

`treefmt` has the following specification:

```
One CLI to format your repo

Usage:
  treefmt <paths...> [flags]

Flags:
      --allow-missing-formatter   Do not exit with error if a configured formatter is missing. (env $TREEFMT_ALLOW_MISSING_FORMATTER)
      --ci                        Runs treefmt in a CI mode, enabling --no-cache, --fail-on-change and adjusting some other settings best suited to a CI use case. (env $TREEFMT_CI)
  -c, --clear-cache               Reset the evaluation cache. Use in case the cache is not precise enough. (env $TREEFMT_CLEAR_CACHE)
      --config-file string        Load the config file from the given path (defaults to searching upwards for treefmt.toml or .treefmt.toml).
      --cpu-profile string        The file into which a cpu profile will be written. (env $TREEFMT_CPU_PROFILE)
  -e, --excludes strings          Exclude files or directories matching the specified globs. (env $TREEFMT_EXCLUDES)
      --fail-on-change            Exit with error if any changes were made. Useful for CI. (env $TREEFMT_FAIL_ON_CHANGE)
  -f, --formatters strings        Specify formatters to apply. Defaults to all configured formatters. (env $TREEFMT_FORMATTERS)
  -h, --help                      help for treefmt
  -i, --init                      Create a treefmt.toml file in the current directory.
      --no-cache                  Ignore the evaluation cache entirely. Useful for CI. (env $TREEFMT_NO_CACHE)
  -u, --on-unmatched string       Log paths that did not match any formatters at the specified log level. Possible values are <debug|info|warn|error|fatal>. (env $TREEFMT_ON_UNMATCHED) (default "warn")
      --stdin                     Format the context passed in via stdin.
      --tree-root string          The root directory from which treefmt will start walking the filesystem (defaults to the directory containing the config file). (env $TREEFMT_TREE_ROOT)
      --tree-root-file string     File to search for to find the tree root (if --tree-root is not passed). (env $TREEFMT_TREE_ROOT_FILE)
  -v, --verbose count             Set the verbosity of logs e.g. -vv. (env $TREEFMT_VERBOSE)
      --version                   version for treefmt
      --walk string               The method used to traverse the files within the tree root. Currently supports 'auto', 'git' or 'filesystem'. ($TREEFMT_WALK) (default "auto")
  -C, --working-dir string        Run as if treefmt was started in the specified working directory instead of the current working directory. $(TREEFMT_WORKING_DIR) (default ".")
```

## Arguments

### `<paths>...`

Paths to format. Defaults to formatting the whole tree

## Flags

### `--allow-missing-formatter`

`$TREEFMT_ALLOW_MISSING_FORMATTER`

Do not exit with an error if some of the configured formatters are missing.

### `--ci`

`$TREEFMT_CI`

Runs treefmt in a CI mode which does the following:

-   ensures `INFO` level logging at a minimum
-   enables `--no-cache` and `--fail-on-change`
-   introduces a small startup delay so we do not start processing until the second after the process started, thereby
    ensuring the accuracy of our change detection based on second-level `modtime`.

### `--clear-cache`

`$TREEFMT_CLEAR_CACHE`

Reset the evaluation cache. Use in case the cache is not precise enough.

### `--config-file <config-file>`

Load the config file from the given path (defaults to searching upwards for treefmt.toml or .treefmt.toml).

### `--cpu-profile`

`$TREEFMT_CPU_PROFILE`

The file into which a cpu profile will be written.

### `-e, --excludes strings`

`$TREEFMT_EXCLUDES`

Exclude files or directories matching the specified globs.

### `--fail-on-change`

`$TREEFMT_FAIL_ON_CHANGE`

Exit with error if any changes were made. Useful for CI.

### `-f, --formatters strings`

`$TREEFMT_FORMATTERS`

Specify formatters to apply. Defaults to all configured formatters.

### `-h, --help`

Prints available flags and options

### `--init`

Create a treefmt.toml file in the current directory.

### `--no-cache`

`$TREEFMT_NO_CACHE`

Tells `treefmt` to ignore the evaluation cache entirely.

With this flag, you can avoid cache invalidation issues, if any.
Typically, the machine that is running `treefmt` in CI is starting with a fresh environment each time,
so any calculated cache is lost.

### `-u --on-unmatched`

`$TREEFMT_ON_UNMATCHED`

Log paths that did not match any formatters at the specified log level, with fatal exiting the process with an error. Possible values are <debug|info|warn|error|fatal>.

[default: warn]

### `--stdin`

Format the context passed in via stdin.

### `--tree-root string`

`$TREEFMT_TREE_ROOT`

The root directory from which `treefmt` will start walking the filesystem.

[default: the directory containing the config file]

### `--tree-root-file string`

`$TREEFMT_TREE_ROOT_FILE`

File to search for to find the tree root (if `--tree-root` is not passed).

### `-v, --verbose`

Set the verbosity of logs e.g. `-vv`. Can also be set with an integer value in an env variable `$LOG_LEVEL`.

Log verbosity is based off the number of 'v' used. With one `-v`, your logs will display `[INFO]` and `[ERROR]` messages,
while `-vv` will also show `[DEBUG]` messages.

### `--version`

Print version.

### `--walk <auto|git|filesystem>`

`$TREEFMT_WALK`

The method used to traverse the files within `--tree-root`.
Currently, supports `auto`, `git` or `filesystem`.

Default is `auto`, where we will detect if the `<tree-root>` is a git repository and use the `git` walker for
traversal.
If not we will fall back to the `filesystem` walker.

### `-C, --working-directory string`

`$TREEFMT_WORKING_DIR`

Run as if `treefmt` was started in the specified working directory instead of the current working directory

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
