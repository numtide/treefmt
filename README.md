<h1 align="center">
  <br>
  <img src="docs/public/logo.svg" alt="logo" width="200">
  <br>
  treefmt — one CLI to format your repo
  <br>
  <br>
</h1>

[![Support room on Matrix](https://img.shields.io/matrix/treefmt:numtide.com.svg?label=%23treefmt%3Anumtide.com&logo=matrix&server_fqdn=matrix.numtide.com)](https://matrix.to/#/#treefmt:numtide.com)

**Status: beta**

`treefmt` streamlines the process of applying formatters to your project, making it a breeze with just one command line.

## Motivation

It’s common to format code according to the project’s standards before making contributions to any project. This task
seems trivial at first sight — you can set up the required language formatter in your IDE.

However, contributing to multiple projects requires more effort: you must change your formatter configurations each
time you switch between projects or call them manually.

Formatting requires less effort with a universal formatter that supports multiple languages but is still project-specific.

## About treefmt

`treefmt` runs all your formatters with one command. It’s easy to configure and fast to execute.

![Treefmt Init](./docs/public/init.gif)

Its main features are:

-   **Providing a unified CLI and output**
    -   You don’t need to remember which formatters are necessary for each project.
    -   Once you specify the formatters in the config file, you can trigger all of them with one command and get a
        standardized output.
-   **Running all the formatters in parallel**
    -   A standard script loops over your folders and runs each formatter sequentially.
    -   In contrast, `treefmt` runs formatters in parallel. This way, the formatting job takes less time.
-   **Tracking file changes**
    -   When formatters are run in a script, they process all the files they encounter, regardless of whether or not
        they have changed.
    -   `treefmt` tracks file changes, and only attempts to format files which have changed.

To reformat the whole source tree, just type `treefmt` in any folder. This is a fast and simple formatting solution.

## Installation

You can install `treefmt` by downloading the binary. Find the binaries for different architectures [here](https://github.com/numtide/treefmt/releases).
Otherwise, you can install the package from source code — either with [Go], or with the help of [nix].

We describe the installation process in detail in the [docs].

## Usage

In order to use `treefmt` in your project, make sure the config file `treefmt.toml` is present in the root folder and
is edited to suit your needs.

You can generate it with:

```
$ treefmt --init
```

You can then run `treefmt` in your project root folder like this:

```
$ treefmt
```

To explore the tool’s flags and options, type:

```console
$ treefmt --help
```

Additionally, there's a wrapper called [`treefmt-nix`](https://github.com/numtide/treefmt-nix) for using `treefmt` with [`nix`](https://github.com/NixOS/nix).

## Configuration

Formatters are specified in the config file `treefmt.toml`, which is usually located in the project root folder. The
generic way to specify a formatter is like this:

```
[formatter.<name>]
command = "<formatter-command>"
options = ["<formatter-option-1>"...]
includes = ["<glob>"]
```

For example, if you want to use [nixpkgs-fmt] on your Nix project and rustfmt on your Rust project, then
`treefmt.toml` will look as follows:

```toml
[formatter.nix]
command = "nixpkgs-fmt"
includes = ["*.nix"]

[formatter.rust]
command = "rustfmt"
options = ["--edition", "2018"]
includes = ["*.rs"]
```

Before specifying the formatter in the config, make sure it’s installed.

To find and share existing formatter recipes, take a look at the [docs].

If you are a Nix user, you might also be interested in [treefmt-nix](https://github.com/numtide/treefmt-nix) to use Nix to configure and bring in
formatters.

## Compatibility

`treefmt` works with any formatter that adheres to the [following specification](https://github.com/numtide/treefmt/blob/main/docs/formatter-spec.md).

For instance, you can go for:

-   [clang-format] for C/C++/Java/JavaScript/JSON/Objective-C/Protobuf/C#
-   gofmt for Golang
-   Prettier for JavaScript/HTML/CSS

Find the full list of supported formatters [here](https://treefmt.com/configure.html#supported-formatters).

## IDE Integration

`treefmt` currently has support for vscode via an extension:

-   [treefmt-vscode](https://marketplace.visualstudio.com/items?itemName=ibecker.treefmt-vscode) | [GitHub repo](https://github.com/isbecker/treefmt-vscode)

## Upcoming features

This project is still pretty new. Down the line we also want to add support for:

-   More IDE integration
-   Pre-commit hooks

## Related projects

-   [EditorConfig](https://editorconfig.org/): unifies file indentations configuration on a per-project basis.
-   [prettier](https://prettier.io/): an opinionated code formatter for a number of languages.
-   [Super-Linter](https://github.com/github/super-linter): a project by GitHub to lint all of your code.
-   [pre-commit](https://pre-commit.com/): a framework for managing and maintaining multi-language pre-commit hooks.

## Contributing

All contributions are welcome! We try to keep the project simple and focused. Please refer to the [Contributing](./docs/contributing.md)
guidelines for more information.

## Moving from Rust To Go

You may be familiar with [Version 1], which is written in [Rust]. So, why re-write it in [Go]?

Ultimately, `treefmt` is spending most of it's time shelling out calls to the underlying formatters. This process is
just as fast/performant in Go as it is in Rust.

The remaining tasks are processing some cli args and parsing a config file. Do we really need something as _heavy duty_
as Rust for that?

Despite all this, you can make good, sane arguments for continuing with [Version 1] in Rust instead of a re-write.
So here's a _bad argument_.

[Brian] wanted to improve performance by moving away from a [Toml] cache file, introduce pipelines for applying multiple
formatters against the same file set, and add an extensible approach for how `treefmt` walks file systems. He knows Go
much better than Rust.

[zimbatm] thought it was a good idea too.

So here we are :shrug:.

![Just Use Go](./docs/public/just-use-go.png)

## Commercial support

Looking for help or customization?

Get in touch with [Numtide](https://numtide.com/) to get a quote. We make it easy for companies to work with Open
Source projects: <https://numtide.com/contact>

## License

Unless explicitly stated otherwise, any contribution intentionally submitted for inclusion will be licensed under the
[MIT license](LICENSE) without any additional terms or conditions.

[Brian]: https://github.com/brianmcgee
[zimbatm]: https://github.com/zimbatm
[Version 1]: https://github.com/numtide/treefmt/tree/v1
[Rust]: https://www.rust-lang.org/
[Go]: https://go.dev/
[Toml]: https://toml.io/en/
[docs]: https://treefmt.com
[nix]: https://github.com/NixOS/nix
[nixpkgs-fmt]: https://github.com/nix-community/nixpkgs-fmt
[clang-format]: https://clang.llvm.org/docs/ClangFormat.html
