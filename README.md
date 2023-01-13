<h1 align="center">
  <br>
  <img src="docs/assets/logo.svg" alt="logo" width="200">
  <br>
  treefmt — one CLI to format your repo
  <br>
  <br>
</h1>

[![Support room on Matrix](https://img.shields.io/matrix/treefmt:numtide.com.svg?label=%23treefmt%3Anumtide.com&logo=matrix&server_fqdn=matrix.numtide.com)](https://matrix.to/#/#treefmt:numtide.com)

**Status: beta**

`treefmt` applies all the needed formatters to your project with one command line.

## Motivation

Before making contributions to any project, it’s common to get your code formatted according to the project’s standards. This task seems trivial from the first sight — you can simply set up the required language formatter in your IDE. But contributing to multiple projects requires more effort: you need to change the code formatter configs each time you switch between projects, or call formatters manually.

Formatting requires less effort if a universal formatter for multiple languages is in place, which is also project-specific.

## About treefmt

`treefmt` runs all your formatters with one command. It’s easy to configure and fast to execute.

[![asciicast](https://asciinema.org/a/cwtaWUTdBa8qCKJVp40bTwxf0.svg)](https://asciinema.org/a/cwtaWUTdBa8qCKJVp40bTwxf0)

Its main features are:

- Providing a unified CLI and output: You don’t need to remember which formatters are required for each project. Once you specify the formatters in the config file, you can trigger all of them with one command and get a standardized output.
- Running all the formatters in parallel: A standard script loops over your folders and runs each formatter consequentially. In contrast, `treefmt` runs formatters in parallel. This way, the formatting job takes less time.
- Caching the changed files: When formatters are run in a script, they process all the files they encounter, no matter whether the code has changed. This unnecessary work can be eliminated if only the changed files are formatted. `treefmt` caches the changed files and marks them for re-formatting.

Just type `treefmt` in any folder to reformat the whole code tree. All in all, you get a fast and simple formatting solution.

## Installation

You can install the tool by downloading the binary. Find the binaries for different architectures [here](https://github.com/numtide/treefmt/releases). Otherwise, you can install the package from the source code — either with [cargo](https://github.com/rust-lang/cargo), or with help of [nix](https://github.com/NixOS/nix). We describe the installation process in detail in the [wiki](https://github.com/numtide/treefmt/wiki).

## Usage

In order to use `treefmt` in your project, make sure the config file `treefmt.toml` is present in the root folder and is edited to your needs. You can generate it with:

```
$ treefmt --init
```

You can run `treefmt` in your project root folder like this:

```
$ treefmt
```

To explore the tool’s flags and options, type:

```console
$ treefmt --help
```


## Configuration

Fomatters are specified in the config file `treefmt.toml`, which is usually located in the project root folder. The generic way to specify a formatter is like this:

```
[formatter.<name>]
command = "<formatter-command>"
options = [“<formatter-option-1>”...]
includes = ["<glob>"]
```

For example, if you want to use [nixpkgs-fmt](https://github.com/nix-community/nixpkgs-fmt) on your Nix project and rustfmt on your Rust project, then `treefmt.toml` will look as follows:

```
[formatter.nix]
command = "nixpkgs-fmt"
includes = ["*.nix"]

[formatter.rust]
command = "rustfmt"
options = ["--edition", "2018"]
includes = ["*.rs"]
```

Before specifying the formatter in the config, make sure it’s installed.

To find and share existing formatter recipes, take a look at the [wiki](https://github.com/numtide/treefmt/wiki).

If you are a Nix user, you might also be interested in [treefmt-nix](https://github.com/numtide/treefmt-nix) to use Nix to configure and bring in formatters.

## Compatibility

`Treefmt` works with any formatter that adheres to the [following specification](https://github.com/renoire/treefmt/blob/master/docs/formatters-spec.md). For instance, you can go for:

- [clang-format](https://clang.llvm.org/docs/ClangFormat.html) for Java
- gofmt for Golang
- Prettier for JavaScript/HTML/CSS

Find the full list of supported formatters [here](https://numtide.github.io/treefmt/formatters.html).

## Upcoming features

This project is still pretty new. Down the line we also want to add support for:

- IDE integration
- Pre-commit hooks
- Effective support of multiple formatters

## Related projects

- [EditorConfig](https://editorconfig.org/): unifies file indentations
  configuration on a per-project basis.
- [prettier](https://prettier.io/): an opinionated code formatter for a number of languages.
- [Super-Linter](https://github.com/github/super-linter): a project by GitHub to lint all of your code.
- [pre-commit](https://pre-commit.com/): a framework for managing and
  maintaining multi-language pre-commit hooks.

## Contributing

All contributions are welcome! We try to keep the project simple and focused. Please refer to [Contributing](./docs/contributing.md) guidelines for more information.

## License

Unless explicitly stated otherwise, any contribution intentionally submitted for inclusion will be licensed under the [MIT license](LICENSE.md) without any additional terms or conditions.
