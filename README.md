<h1 align="center">
  <br>
  <img src="assets/logo.svg" alt="logo" width="200">
  <br>
  treefmt - one CLI to format the code tree
  <br>
  <br>
</h1>

[![Support room on Matrix](https://img.shields.io/matrix/treefmt:numtide.com.svg?label=%23treefmt%3Anumtide.com&logo=matrix&server_fqdn=matrix.numtide.com)](https://matrix.to/#/#treefmt:numtide.com)

**Status: experimental** -- not all the features described here are working
yet.

Every project has different languages and code formatters, with possibly
different configurations. When jumping between project it always takes a bit
of time to get accustomed to them and update our editor configuration.

This project solves that problem by proposing a unified CLI interface that
traverses the project file tree, and maps each file to a different code
formatter. Type `treefmt` and it re-formats the repository.

Treefmt also includes some file traversal optimizations which should make the
common invocation faster.

## Design decisions

We assume that project code is checked into source control. Therefor, the
default should be to write formatter changes back in place. Options like
`--dry-run` are not needed, the source control is relied upon to revert or
check for code changes.

`treefmt` is responsible for traversing the file-system and mapping files to
specific code formatters.

Only *one* formatter per file. `treefmt` enforces that only one tool is
executed per file. Guaranteeing two tools to product idempotent outputs is
quite difficult.

## Usage

```
treefmt [options] [<file>...]
```

* `file`: path to files to format. If no files are passed, format all of the
          files from the current folder and down.

### Options

* `--init`: Creates a templated `treefmt.toml` in the current directory.

* `--config <path>`: Overrides the `treefmt.toml` file lookup.

* `--help`: Shows this help.

## Configuration format

`treefmt` depends on the `treefmt.toml` to map file extensions to actual code
formatters. That file is searched for recursively from the current folder and
up unless the `--config <path>` option is passed.

### `[formatters.<name>]`

This section describes the integration between a single formatter and
`treefmt`.

* `command`: A list of arguments to execute the formatter. This will be
    composed with the `options` attribute during invocation. The first argument
    is the name of the executable to run.

* `options`: A list of extra arguments to add to the command. This is typically
    project-specific arguments.

* `includes`: A list of glob patterns used to select files. Usually this would be
    something like `[ "*.sh" ]` to select all the shell scripts. Sometimes,
    full filenames can be passed. Eg: `[ "Makefile" ]`.

* `excludes`: A list of glob patterns to deny. If any of these patterns match,
    the file will be excluded.

## Use cases

### CLI usage

As a developer, I want to run `treefmt` in any folder and it would
automatically format all of the code, configured for the project. I don't want
to remember what tool to use, or their magic incantation.

### Editor integration

TODO: not supported yet.

Editors often want to be able to format a file, before it gets written to disk.

Ideally, the editor would pipe the code in, pass the filename, and get the
formatted code out. Eg: `cat ./my_file.sh | treefmt --stdin my_file.sh >
formatted_file.sh`

### CI integration

We can assume that code lives in a source control.

For example a Git integration would look like this:

```sh
#!/usr/bin/env bash
set -euo pipefail

# Format all of the code
treefmt

# Check that there are no changes in the code
if [[ -n "$(git status --porcelain -unormal)" ]]; then
  echo "Some code needs formatting! Please run \`treefmt\`.
  git status -unormal
  exit 1
fi
echo "OK"
```

## Interfaces

In order to keep the design of treefmt simple, we ask code formatters to
adhere to the following specification.

[treefmt formatter spec](docs/formatter_spec.md)

If they don't, the best is to create a wrapper script that transforms the
usage to match that spec.

## Related projects

* [EditorConfig](https://editorconfig.org/): unifies file indentations
  configuration on a per-project basis.
* [prettier](https://prettier.io/): and opinionated code formatter for a
    number of languages.

## Contributing

All contributions are welcome! We try to keep the project simple and focused
so not everything will be accepted. Please open an issue to discuss before
working on a big item.

If you want to discuss, we have a public Matrix channel:
[#treefmt:numtide.com](https://matrix.to/#/#treefmt:numtide.com)

## License

MIT - (c) 2020 NumTide Ltd.
