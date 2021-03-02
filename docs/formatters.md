# A list of known formatters

Here is a list of all the formatters we tested. Feel free to send a PR to add
other ones!

## [prettier](https://prettier.io/)

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

## [Black](https://github.com/psf/black)

A python formatter.

```toml
command = "black"
includes = ["*.py"]
```

## Elm

```toml
command = "elm-format"
options = ["--yes"]
includes = ["*.elm"]
```

## Go

```toml
command = "gofmt"
options = ["-w"]
includes = ["*.go"]
```

## [Ormolu](https://github.com/tweag/ormolu)

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

## [nixpkgs-fmt](https://github.com/nix-community/nixpkgs-fmt)

Nix code formatter.

```toml
command = "nixpkgs-fmt"
includes = ["*.nix"]
```

## rustfmt

```toml
command = "rustfmt"
options = ["--edition", "2018"]
includes = ["*.rs"]
```

## cargo fmt

`cargo fmt` is not supported as it doesn't follow the spec. It doesn't allow
to pass arbitrary files to be formatted, which treefmt relies on.

## [shfmt](https://github.com/mvdan/sh)

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
