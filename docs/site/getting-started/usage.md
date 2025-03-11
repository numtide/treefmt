---
outline: deep
---

# Usage

`treefmt` has the following specification:

```
Usage:
  treefmt <paths...> [flags]

Flags:
      --allow-missing-formatter   Do not exit with error if a configured formatter is missing. (env $TREEFMT_ALLOW_MISSING_FORMATTER)
      --ci                        Runs treefmt in a CI mode, enabling --no-cache, --fail-on-change and adjusting some other settings best suited to a CI use case. (env $TREEFMT_CI)
  -c, --clear-cache               Reset the evaluation cache. Use in case the cache is not precise enough. (env $TREEFMT_CLEAR_CACHE)
      --completion string         [bash|zsh|fish] Generate shell completion scripts for the specified shell.
      --config-file string        Load the config file from the given path (defaults to searching upwards for treefmt.toml or .treefmt.toml).
      --cpu-profile string        The file into which a cpu profile will be written. (env $TREEFMT_CPU_PROFILE)
      --excludes strings          Exclude files or directories matching the specified globs. (env $TREEFMT_EXCLUDES)
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
      --walk string               The method used to traverse the files within the tree root. Currently supports <auto|git|filesystem>. (env $TREEFMT_WALK) (default "auto")
  -C, --working-dir string        Run as if treefmt was started in the specified working directory instead of the current working directory. (env $TREEFMT_WORKING_DIR) (default ".")
```

Typically, you will execute `treefmt` from the root of your repository with no arguments:

```console
❯ treefmt
traversed 106 files
emitted 9 files for processing
formatted 6 files (2 changed) in 184ms
```

## Clear Cache

To force re-evaluation of the entire tree, you run `treefmt` with the `-c` or `--clear-cache` flag:

```console
❯ treefmt -c
traversed 106 files
emitted 106 files for processing
formatted 56 files (0 changed) in 363ms

❯ treefmt --clear-cache
traversed 106 files
emitted 106 files for processing
formatted 56 files (0 changed) in 351ms
```

## Change working directory

Similar to [git](https://git-scm.com/), `treefmt` has an option to [change working directory](./configure.md#working-dir)
before executing:

```console
❯ treefmt -C test/examples --allow-missing-formatter
traversed 106 files
emitted 56 files for processing
formatted 46 files (1 changed) in 406ms
```

## Format files & directories

To format one or more specific files, you can pass them as arguments.

```console
> treefmt default.nix walk/walk.go nix/devshells/renovate.nix
traversed 3 files
emitted 3 files for processing
formatted 3 files (0 changed) in 144ms
```

You can also pass directories:

```console
> treefmt nix walk/cache
traversed 9 files
emitted 8 files for processing
formatted 7 files (0 changed) in 217ms
```

!!!note

    When passing directories as arguments, `treefmt` will traverse them using the configured [walk](./configure.md#walk)
    strategy.

## Format stdin

Using the [stdin](./configure.md#stdin) option, `treefmt` can format content passed via `stdin`, forwarding its
output to `stdout`:

```console
❯ cat default.nix | treefmt --stdin foo.nix
# This file provides backward compatibility to nix < 2.4 clients
{system ? builtins.currentSystem}: let
  lock = builtins.fromJSON (builtins.readFile ./flake.lock);

  inherit
    (lock.nodes.flake-compat.locked)
    owner
    repo
    rev
    narHash
    ;

  flake-compat = fetchTarball {
    url = "https://github.com/${owner}/${repo}/archive/${rev}.tar.gz";
    sha256 = narHash;
  };

  flake = import flake-compat {
    inherit system;
    src = ./.;
  };
in
  flake.defaultNix
```

## Shell Completion

To generate completions for your preferred shell:

```console
❯ treefmt --completion bash
❯ treefmt --completion fish
❯ treefmt --completion zsh
```

## CI integration

We recommend using the [CI option](./configure.md#ci) in continuous integration environments.

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
