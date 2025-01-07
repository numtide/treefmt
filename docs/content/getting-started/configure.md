# Configure

`treefmt`'s behaviour can be influenced in one of three ways:

1. Process flags and arguments
2. Environment variables
3. A [TOML] based config file

There is an order of precedence between these mechanisms as listed above, with process flags having the highest
precedence and values in the configuration file having the lowest.

!!!note

    Some options can **only be configured as process flags**,
    others may support **process flags and environment variables**,
    and others still may support **all three mechanisms**.

## Config File

The `treefmt` configuration file is a mixture of global options and formatter sections.

It should be named `treefmt.toml` or `.treefmt.toml`, and typically resides at the root of a repository.

When executing `treefmt` within a subdirectory, `treefmt` will search upwards in the directory structure, looking for
`treefmt.toml` or `.treefmt.toml`.
You can change this behaviour using the [config-file](#config-file_1) options

!!! tip

    When starting a new project you can generate an initial config file using `treefmt --init`

```nix title="treefmt.toml"
--8<-- "cmd/init/init.toml"
```

## Global Options

### `allow-missing-formatter`

Do not exit with error if a configured formatter is missing.

=== "Flag"

    ```console
    treefmt --allow-missing-formatter true
    ```

=== "Env"

    ```console
    TREEFMT_ALLOW_MISSING_FORMATTER=true treefmt
    ```

=== "Config"

    ```toml
    allow-missing-formatter = true
    ```

### `ci`

Runs treefmt in a CI mode, enabling [no-cache](#no-cache), [fail-on-change](#fail-on-change) and adjusting some other settings best suited to a
continuous integration environment.

=== "Flag"

    ```console
    treefmt --ci
    ```

=== "Env"

    ```console
    TREEFMT_CI=true treefmt
    ```

### `clear-cache`

Reset the evaluation cache. Use in case the cache is not precise enough.

=== "Flag"

    ```console
    treefmt -c
    treefmt --clear-cache
    ```

=== "Env"

    ```console
    TREEFMT_CLEAR_CACHE=true treefmt
    ```

### `config-file`

=== "Flag"

    ```console
    treefmt --config-file /tmp/treefmt.toml
    ```

=== "Env"

    ```console
    TREEFMT_CONFIG=/tmp/treefmt.toml treefmt
    ```

### `cpu-profile`

The file into which a [pprof](https://github.com/google/pprof) cpu profile will be written.

=== "Flag"

    ```console
    treefmt --cpu-profile ./cpu.pprof
    ```

=== "Env"

    ```console
    TREEFMT_CPU_PROFILE=./cpu.pprof treefmt
    ```

=== "Config"

    ```toml
    cpu-profile = "./cpu.pprof"
    ```

### `excludes`

An optional list of [glob patterns](#glob-patterns-format) used to exclude files from all formatters.

=== "Flag"

    ```console
    treefmt --excludes *.toml,*.php,README
    ```

=== "Env"

    ```console
    TREEFMT_EXCLUDES="*.toml,*.php,README" treefmt
    ```

=== "Config"

    ```toml
    excludes = ["*.toml", "*.php", "README"]
    ```

### `fail-on-change`

Exit with error if any changes were made during execution.

=== "Flag"

    ```console
    treefmt --fail-on-change true
    ```

=== "Env"

    ```console
    TREEFMT_FAIL_ON_CHANGE=true treefmt
    ```

=== "Config"

    ```toml
    fail-on-change = true
    ```

### `formatters`

A list of formatters to apply.
Defaults to all configured formatters.

=== "Flag"

    ```console
    treefmt -f go,toml,haskell
    treefmt --formatters go,toml,haskell
    ```

=== "Env"

    ```console
    TREEFMT_FORMATTERS=go,toml,haskell treefmt
    ```

=== "Config"

    ```toml
    formatters = ["go", "toml", "haskell"]

    [formatter.go]
    ...

    [formatter.toml]
    ...

    [formatter.haskell]
    ...

    [formatter.ruby]
    ...

    [formatter.shellcheck]
    ...
    ```

### `no-cache`

Ignore the evaluation cache entirely. Useful for CI.

=== "Flag"

    ```console
    treefmt --no-cache
    ```

=== "Env"

    ```console
    TREEFMT_NO_CACHE=true treefmt
    ```

### `on-unmatched`

Log paths that did not match any formatters at the specified log level.
Possible values are `<debug|info|warn|error|fatal>`.

!!! warning

    If you select `fatal`, the process will exit immediately with a non-zero exit.

=== "Flag"

    ```console
    treefmt -u debug
    treefmt --on-unmatched debug
    ```

=== "Env"

    ```console
    TREEFMT_ON_UNMACTHED=info treefmt
    ```

=== "Config"

    ```toml
    on-unmatched = "debug"
    ```

### `quiet`

Suppress all output except for errors.

=== "Flag"

    ```console
    treefmt --quiet
    ```

=== "Env"

    ```console
    TREEFMT_QUIET=true treefmt
    ```

### `stdin`

Format the context passed in via stdin.

!!! note
You must provide a single path argument, the value of which is used to match against the configured formatters.

=== "Flag"

    ```console
    cat ../test.go | treefmt --stdin foo.go
    ```

### `tree-root`

The root directory from which treefmt will start walking the filesystem.
Defaults to the directory containing the config file.

=== "Flag"

    ```console
    treefmt --tree-root /tmp/foo
    ```

=== "Env"

    ```console
    TREEFMT_TREE_ROOT=/tmp/foo treefmt
    ```

=== "Config"

    ```toml
    tree-root = "/tmp/foo"
    ```

### `tree-root-file`

File to search for to find the tree root (if `tree-root` is not set)

=== "Flag"

    ```console
    treefmt --tree-root-file .git/config
    ```

=== "Env"

    ```console
    TREEFMT_TREE_ROOT_FILE=.git/config treefmt
    ```

=== "Config"

    ```toml
    tree-root-file = ".git/config"
    ```

### `verbose`

Set the verbosity level of logs:

-   `0` => `warn`
-   `1` => `info`
-   `2` => `debug`

=== "Flag"

    The number of `v`'s passed matches the level set.

    ```console
    treefmt -vv
    ```

=== "Env"

    ```console
    TREEFMT_VERBOSE=1 treefmt
    ```

=== "Config"

    ```toml
    verbose = 2
    ```

### `walk`

The method used to traverse the files within the tree root.
Currently, we support 'auto', 'git' or 'filesystem'

=== "Flag"

    ```console
    treefmt --walk filesystem
    ```

=== "Env"

    ```console
    TREEFMT_WALK=filesystem treefmt
    ```

=== "Config"

    ```toml
    walk = "filesystem"
    ```

### `working-dir`

Run as if `treefmt` was started in the specified working directory instead of the current working directory.

=== "Flag"

    ```console
    treefmt -C /tmp/foo
    treefmt --working-dir /tmp/foo
    ```

=== "Env"

    ```console
    TREEFMT_WORKING_DIR=/tmp/foo treefmt
    ```

## Formatter Options

Formatters are configured using a [table](https://toml.io/en/v1.0.0#table) entry in `treefmt.toml` of the form
`[formatter.<name>]`:

```toml
[formatter.alejandra]
command = "alejandra"
includes = ["*.nix"]
excludes = ["examples/nix/sources.nix"]
priority = 1

[formatter.deadnix]
command = "deadnix"
options = ["-e"]
includes = ["*.nix"]
priority = 2
```

### `command`

The command to invoke when applying the formatter.

### `options`

An optional list of args to be passed to `command`.

### `includes`

A list of [glob patterns](#glob-patterns-format) used to determine whether the formatter should be applied against a given path.

### `excludes`

An optional list of [glob patterns](#glob-patterns-format) used to exclude certain files from this formatter.

### `priority`

Influences the order of execution. Greater precedence is given to lower numbers, with the default being `0`.

## Same file, multiple formatters?

For each file, `treefmt` determines a list of formatters based on the configured `includes` / `excludes` rules. This list is
then sorted, first by priority (lower the value, higher the precedence) and secondly by formatter name (lexicographically).

The resultant sequence of formatters is used to create a batch key, and similarly matched files get added to that batch
until it is full, at which point the files are passed to each formatter in turn.

This means that `treefmt` **guarantees only one formatter will be operating on a given file at any point in time**.
Another consequence is that formatting is deterministic for a given file and a given `treefmt` configuration.

By setting the priority fields appropriately, you can control the order in which those formatters are applied for any
files they _both happen to match on_.

## Glob patterns format

This is a variant of the Unix glob pattern. It supports all the usual
selectors such as `*` and `?`.

### Examples

-   `*.go` - match all files in the project that end with a ".go" file extension.
-   `vendor/*` - match all files under the vendor folder, recursively.

## Supported Formatters

Any formatter that follows the [spec] is supported out of the box.

Already 60+ formatters are supported.

To find examples, take a look at <https://github.com/numtide/treefmt-nix/tree/main/examples>.

If you are a Nix user, you might also like <https://github.com/numtide/treefmt-nix>, which uses Nix to pull in the right formatter package and seamlessly integrates both together.

[spec]: ../reference/formatter-spec.md
[TOML]: https://toml.io
