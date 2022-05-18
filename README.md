<h1 align="center">
  <br>
  <img src="docs/assets/logo.svg" alt="logo" width="200">
  <br>
  treefmt - one CLI to format the code tree
  <br>
  <br>
</h1>

[![Support room on Matrix](https://img.shields.io/matrix/treefmt:numtide.com.svg?label=%23treefmt%3Anumtide.com&logo=matrix&server_fqdn=matrix.numtide.com)](https://matrix.to/#/#treefmt:numtide.com)

**Status: beta**

When working on large code trees, it's common to have multiple code
formatters run against it. And have one script that loops over all of them.
`treefmt` makes that nicer.

- A unified CLI and output
- Run all the formatters in parallel.
- Cache which files have changed for super fast re-formatting.

Just type `treefmt` in any folder and it reformats the whole code tree.

## Design decisions

We assume that the project code is checked into source control. Therefore, the
default should be to write formatter changes back in place. Options like
`--dry-run` are not needed; the source control is relied upon to revert or
check for code changes.

`treefmt` is responsible for traversing the file-system and mapping files to
specific code formatters.

Only _one_ formatter per file. `treefmt` enforces that only one tool is
executed per file. Guaranteeing two tools to produce idempotent outputs is
quite tricky.

## Usage

`$ treefmt --help`

```
treefmt 0.4.1
✨  format all your language!

USAGE:
    treefmt [FLAGS] [OPTIONS] [paths]...

FLAGS:
        --clear-cache       Reset the evaluation cache. Use in case the cache is not precise enough
        --fail-on-change    Exit with error if any changes were made. Useful for CI
    -h, --help              Prints help information
        --init              Create a new treefmt.toml
        --no-cache          Ignore the evaluation cache entirely. Useful for CI
    -q, --quiet             No output printed to stderr
        --stdin             Format the content passed in stdin
    -V, --version           Prints version information
    -v, --verbose           Log verbosity is based off the number of v used

OPTIONS:
        --config-file <config-file>    Run with the specified config file, which is not required to be in the tree to be
                                       formatted
    -f, --formatters <formatters>...   Select formatters name to apply. Defaults to all formatters
        --tree-root <tree-root>        Set the path to the tree root directory. Defaults to the folder holding the
                                       treefmt.toml file
    -C <work-dir>                      Run as if treefmt was started in <work-dir> instead of the current working
                                       directory [default: .]

ARGS:
    <paths>...    Paths to format. Defaults to formatting the whole tree
```

## Configuration format

In order to use `treefmt` in the project, `treefmt.toml` should exists in the root folder. For example, we want to use [nixpkgs-fmt](https://github.com/nix-community/nixpkgs-fmt) on our Nix project and rustfmt on our Rust project, then the `treefmt.toml` will be written as follows:

```
[formatter.nix]
command = "nixpkgs-fmt"
includes = ["*.nix"]

[formatter.rust]
command = "rustfmt"
options = ["--edition", "2018"]
includes = ["*.rs"]
```

See the [wiki](https://github.com/numtide/treefmt/wiki) for more examples.

## Use cases

### CLI usage

As a developer, I want to run `treefmt` in any folder and it would
automatically format all of the code, configured for the project. I don't want
to remember what tool to use, or their magic incantation.

### Editor integration

Editors often want to be able to format a file, before it gets written to disk.

Ideally, the editor would pipe the code in, pass the filename, and get the
formatted code out. Eg: `cat ./my_file.sh | treefmt --stdin my_file.sh > formatted_file.sh`

### CI integration

The `--fail-on-change` flag can be used to exit with error if any files were
re-formatted.

Eg:

```sh
#!/usr/bin/env bash
set -euo pipefail

# Format all of the code and exit with error if there are any changes
treefmt --fail-on-change
```

## Interfaces

In order to keep the design of treefmt simple, we ask code formatters to
adhere to the following specification.

[treefmt formatter spec](./docs/formatters-spec.md)

If they don't, the best is to create a wrapper script that transforms the
usage to match that spec.

## Related projects

- [EditorConfig](https://editorconfig.org/): unifies file indentations
  configuration on a per-project basis.
- [prettier](https://prettier.io/): and opinionated code formatter for a
  number of languages.
- [Super-Linter](https://github.com/github/super-linter): a project by GitHub
  to lint all of the code.
- [pre-commit](https://pre-commit.com/): a framework for managing and
  maintaining multi-language pre-commit hooks.

## Contributing

All contributions are welcome! We try to keep the project simple and focused
so not everything will be accepted. Please refer to
[Contributing](./docs/contributing.md) guidelines for more information.

## License

Unless explicitly stated otherwise, any contribution intentionally submitted for inclusion in the work by you shall be licensed under the [MIT license](LICENSE.md), without any additional terms or conditions.
