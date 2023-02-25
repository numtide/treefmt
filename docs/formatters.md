# Known formatters

Here is a list of all the formatters we tested. Feel free to send a PR to add other ones!

## Contents

Single-language formatters:

- [Cabal](#cabal)
  - [cabal-fmt](#cabal-fmt)
- [Elm](#elm)
  - [elm-format](#elm-format)
- [Golang](#golang)
  - [gofmt](#gofmt)
  - [gofumpt](#gofumpt)
- [Haskell](#haskell)
  - [hlint](#hlint)
  - [ormolu](#ormolu)
  - [stylish-haskell](#stylish-haskell)
- [Lua](#lua)
  - [StyLua](#stylua)
- [Nix](#nix)
  - [alejandra](#alejandra)
  - [nixpkgs-fmt](#nixpkgs-fmt)
- [OCaml](#ocaml)
  - [ocamlformat](#ocamlformat)
- [PureScript](#purescript)
  - [purs-tidy](#purs-tidy)
- [Python](#python)
  - [black](#black)
- [Ruby](#ruby)
  - [rufo](#rufo)
- [Rust](#rust)
  - [rustfmt](#rustfmt)
- [Scala](#scala)
  - [scalafmt](#scalafmt)
- [Shell](#shell)
  - [shellcheck](#shellcheck)
  - [shfmt](#shfmt)
- [Terraform](#terraform)
  - [terraform fmt](#terraform-fmt)

Multilanguage formatters:

- [clang-format](#clang-format)
- [Prettier](#prettier)

## Cabal

### [cabal-fmt](https://github.com/phadej/cabal-fmt)

```
command = "cabal-fmt"
options = ["--inplace"]
includes = ["*.cabal"]
```

## Elm

### [elm-format](https://numtide.github.io/treefmt/formatters.html#elm)

```
command = "elm-format"
options = ["--yes"]
includes = ["*.elm"]
```

## Golang

### [gofmt](https://pkg.go.dev/cmd/gofmt)

```
command = "gofmt"
options = ["-w"]
includes = ["*.go"]

```

### [gofumpt](https://github.com/mvdan/gofumpt)

```
command = "gofumpt"
includes = ["*.go"]

```

## Haskell

### [hlint](https://github.com/ndmitchell/hlint)

```
command = "hlint"
includes = [ "*.hs" ]
```

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

## Lua

### [StyLua](https://github.com/JohnnyMorganz/StyLua)

```
command = "stylua"
includes = ["*.lua"]
```

## Nix

### [Alejandra](https://github.com/kamadorueda/alejandra)

```
command = "alejandra"
includes = ["*.nix"]
```

### [nixpkgs-fmt](https://github.com/nix-community/nixpkgs-fmt)

```
command = "nixpkgs-fmt"
includes = ["*.nix"]
```

## OCaml

### [ocamlformat](https://github.com/ocaml-ppx/ocamlformat)

```
command = "ocamlformat"
options = ["-i"]
includes = ["*.ml", "*.mli"]
```

## PureScript

### [purs-tidy](https://www.npmjs.com/package/purs-tidy)

```
command = "purs-tidy"
includes = ["*.purs"]
```

## Python

### [black](https://github.com/psf/black)

```
command = "black"
includes = ["*.py"]
```

## Ruby

### [rufo](https://github.com/ruby-formatter/rufo)

Rufo is an opinionated ruby formatter. By default it exits with status 3 on file change so you have to pass the `-x` option.

```
command = "rufo"
options = ["-x"]
includes = ["*.rb"]
```

## Rust

cargo fmt is not supported as it doesn't follow the spec. It doesn't allow to pass arbitrary files to be formatter, an ability which `treefmt` relies on. Use rustfmt instead (which is what cargo fmt uses under the hood).

### [rustfmt](https://github.com/rust-lang/rustfmt)

```
command = "rustfmt"
options = ["--edition", "2018"]
includes = ["*.rs"]
```

## Scala

### [scalafmt](https://github.com/scalameta/scalafmt)

```
command = "scalafmt"
includes = ["*.scala"]
```

## Shell

### [shellcheck](https://github.com/koalaman/shellcheck)

```
command = "shellcheck"
includes = ["*.sh"]
```

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
