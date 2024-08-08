---
outline: deep
---

# Configure Treefmt

The `treefmt.toml` configuration file consists of a mixture of global options and formatter sections:

```toml
[global]
excludes = ["*.md", "*.dat"]

[formatter.elm]
command = "elm-format"
options = ["--yes"]
includes = ["*.elm"]

[formatter.go]
command = "gofmt"
options = ["-w"]
includes = ["*.go"]

[formatter.python]
command = "black"
includes = ["*.py"]

# use the priority field to control the order of execution

# run shellcheck first
[formatter.shellcheck]
command = "shellcheck"
includes = ["*.sh"]
priority = 0    # default is 0, but we set it here for clarity

# shfmt second
[formatter.shfmt]
command = "shfmt"
options = ["-s", "-w"]
includes = ["*.sh"]
priority = 1
```

## Global Options

-   `excludes` - an optional list of [glob patterns](#glob-patterns-format) used to exclude certain files from all formatters.

## Formatter Options

-   `command` - the command to invoke when applying the formatter.
-   `options` - an optional list of args to be passed to `command`.
-   `includes` - a list of [glob patterns](#glob-patterns-format) used to determine whether the formatter should be applied against a given path.
-   `excludes` - an optional list of [glob patterns](#glob-patterns-format) used to exclude certain files from this formatter.
-   `priority` - influences the order of execution. Greater precedence is given to lower numbers, with the default being `0`.

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

[spec]: formatter-spec.md
