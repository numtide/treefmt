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

# run shellcheck first
[formatter.shellcheck]
command = "shellcheck"
includes = ["*.sh"]
pipeline = "sh"
priority = 0

# shfmt second
[formatter.shfmt]
command = "shfmt"
options = ["-s", "-w"]
includes = ["*.sh"]
pipeline = "sh"
priority = 1
```

## Global Options

-   `excludes` - an optional list of glob patters used to exclude certain files from all formatters.

## Formatter Options

-   `command` - the command to invoke when applying the formatter.
-   `options` - an optional list of args to be passed to `command`.
-   `includes` - a list of glob patterns used to determine whether the formatter should be applied against a given path.
-   `excludes` - an optional list of glob patterns used to exclude certain files from this formatter.
-   `pipeline` - an optional key used to group related formatters together, ensuring they are executed sequentially
    against a given path.
-   `priority` - indicates the order of execution when this formatter is operating as part of a pipeline. Greater
    precedence is given to lower numbers, with the default being `0`.

> When two or more formatters in a pipeline have the same priority, they are executed in lexicographical order to
> ensure deterministic behaviour over multiple executions.

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
