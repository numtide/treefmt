---
outline: deep
---

# Configure Treefmt

The `treefmt.toml` configuration file consists of a mixture of global options and formatter sections:

```toml
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

-   `excludes` - an optional list of glob patters used to exclude certain files from all formatters.

## Formatter Options

-   `command` - the command to invoke when applying the formatter.
-   `options` - an optional list of args to be passed to `command`.
-   `includes` - a list of glob patterns used to determine whether the formatter should be applied against a given path.
-   `excludes` - an optional list of glob patterns used to exclude certain files from this formatter.
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

## Supported Formatters

Here is a list of all the formatters we tested. Feel free to send a PR to add other ones!

### [prettier](https://prettier.io/)

An opinionated code formatter that supports many languages.

```toml
command = "prettier"
options = ["--write"]
includes = [
    "*.css",
    "*.html",
    "*.js",
    "*.json",
    "*.jsx",
    "*.md",
    "*.mdx",
    "*.scss",
    "*.ts",
    "*.yaml",
]
```

### [Black](https://github.com/psf/black)

A python formatter.

```toml
command = "black"
includes = ["*.py"]
```

### [clang-format](https://clang.llvm.org/docs/ClangFormat.html)

A tool to format C/C++/Java/JavaScript/Objective-C/Protobuf/C# code.

```toml
command = "clang-format"
options = [ "-i" ]
includes = [ "*.c", "*.cpp", "*.cc", "*.h", "*.hpp" ]
```

Note: This example focuses on C/C++ but can be modified to use with other languages.

### Elm

```toml
command = "elm-format"
options = ["--yes"]
includes = ["*.elm"]
```

### Go

```toml
command = "gofmt"
options = ["-w"]
includes = ["*.go"]
```

### [Ormolu](https://github.com/tweag/ormolu)

Haskell formatter. Make sure to use ormolu 0.1.4.0+ as older versions don't
adhere to the spec.

```toml
command = "ormolu"
options = [
    "--ghc-opt", "-XBangPatterns",
    "--ghc-opt", "-XPatternSynonyms",
    "--ghc-opt", "-XTypeApplications",
    "--mode", "inplace",
    "--check-idempotence",
]
includes = ["*.hs"]
```

### [stylish-haskell](https://github.com/jaspervdj/stylish-haskell)

Another Haskell formatter.

```toml
command = "stylish-haskell"
options = [ "--inplace" ]
includes = [ "*.hs" ]
```

### [nixpkgs-fmt](https://github.com/nix-community/nixpkgs-fmt)

Nix code formatter.

```toml
command = "nixpkgs-fmt"
includes = ["*.nix"]
```

### rustfmt

```toml
command = "rustfmt"
options = ["--edition", "2018"]
includes = ["*.rs"]
```

### [rufo](https://github.com/ruby-formatter/rufo)

Rufo is an opinionated ruby formatter. By default it exits with status 3 on
file change so we have to pass the `-x` option.

```toml
command = "rufo"
options = ["-x"]
includes = ["*.rb"]
```

### cargo fmt

`cargo fmt` is not supported as it doesn't follow the spec. It doesn't allow
to pass arbitrary files to be formatted, which treefmt relies on. Use `rustfmt`
instead (which is what `cargo fmt` uses under the hood).

### [shfmt](https://github.com/mvdan/sh)

A shell code formatter.

```toml
command = "shfmt"
options = [
  "-i",
  "2",  # indent 2
  "-s",  # simplify the code
  "-w",  # write back to the file
]
includes = ["*.sh"]
```

### terraform

terraform fmt only supports formatting one file at the time. See
https://github.com/hashicorp/terraform/pull/28191
