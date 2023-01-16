# Known formatters

Here is a list of all the formatters we tested. Feel free to send a PR to add other ones!

## Python

### [Black](https://github.com/psf/black)

```
command = "black"
includes = ["*.py"]
```

## Elm

### [Elm-format](https://numtide.github.io/treefmt/formatters.html#elm)

```
command = "elm-format"
options = ["--yes"]
includes = ["*.elm"]
```

## Golang

### [Gofmt](https://pkg.go.dev/cmd/gofmt)

```
command = "gofmt"
options = ["-w"]
includes = ["*.go"]
```

## Haskell

### [Ormolu](https://github.com/tweag/ormolu)

Make sure to use ormolu 0.1.4.0+ as older versions don't adhere to the spec.

```
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

```
command = "stylish-haskell"
options = [ "--inplace" ]
includes = [ "*.hs" ]
```

## Nix

### [nixpkgs-fmt](https://github.com/nix-community/nixpkgs-fmt)

```
command = "nixpkgs-fmt"
includes = ["*.nix"]
```

## Rust

cargo fmt is not supported as it doesn't follow the spec. It doesn't allow to pass arbitrary files to be formatter, an ability which `treefmt` relies on. Use rustfmt instead (which is what cargo fmt uses under the hood).

### [rustfmt](https://github.com/rust-lang/rustfmt)

```
command = "rustfmt"
options = ["--edition", "2018"]
includes = ["*.rs"]
```

## Ruby

### [rufo](https://github.com/ruby-formatter/rufo)

Rufo is an opinionated ruby formatter. By default it exits with status 3 on file change so you have to pass the `-x` option.

```
command = "rufo"
options = ["-x"]
includes = ["*.rb"]
```

## Shell

### [shfmt](https://github.com/mvdan/sh)

```
command = "shfmt"
options = [
 "-i",
 "2",  # indent 2
 "-s",  # simplify the code
 "-w",  # write back to the file
]
includes = ["*.sh"]
```

## Terraform

### [terraform](https://numtide.github.io/treefmt/formatters.html#terraform)

Make sure to use terraform 1.3.0 or later versions, as earlier versions format only one file at a time. See the details [here](https://github.com/hashicorp/terraform/pull/28191).

```
command = "terraform"
options = ["fmt"]
includes = ["*.tf"]
```

## Multi-language formatters

### [clang-format](https://clang.llvm.org/docs/ClangFormat.html)

A tool to format C/C++/Java/JavaScript/Objective-C/Protobuf/C# code.

```
command = "clang-format"
options = [ "-i" ]
includes = [ "*.c", "*.cpp", "*.cc", "*.h", "*.hpp" ]
```

**Note:** This example focuses on C/C++ but can be modified to be used with other languages.

### [Prettier](https://prettier.io/)

An opinionated code formatter that supports many languages.

```
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
