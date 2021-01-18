# prjfmt - the unified CLI code formatters for projects

**Status: experimental** -- not all the features described here are working
yet.

Every project has different languages and code formatters, with possibly
different configurations. When jumping between project it always takes a bit
of time to get accustomed to them and update our editor configuration.

This project tries to solve that by proposing a unified CLI interface: type
`prjfmt` to format all of the files in a given project, using the project's
configuration.

## Design decisions

We assume that project code is checked into source control. Therefor, the
default should be to write formatter changes back in place. Options like
`--dry-run` are not needed, we rely on the source control to revert or check
for code changes.

`prjfmt` is responsible for traversing the file-system and mapping files to
specific code formatters.

We want *all* of the project files to be formatted. `prjfmt` should hint if
any files in the project are not covered.

Only *one* formatter per file. `prjfmt` should enforces that only one tool is
executed per file. Guaranteeing two tools to product idempotent outputs is
quite difficult.

## Usage

```
prjfmt [options] [<file>...]
```

* `file`: path to files to format. If no files are passed, format all of the
          files from the current folder and down.

### Options

* `--stdin`: If this option is passed, only a single `file` must be passed to
  the command-line. Instead of reading the file, it will read the content from
  stdin, and write the formatted result to stdout. The `file` is only used to
  select the right formatter.

* `--init`: Creates a templated `prjfmt.toml` in the current directory. In the
    future we want to add some heuristic for detecting the languages available
    in the project and make some suggestions for it.

* `--show-config`: Prints out the merged configuration for the current folder.

* `--help`: Shows this help.

## Configuration format

`prjfmt` depends on the `prjfmt.toml` to map file extensions to actual code
formatters. That file is searched for recursively from the current folder and
up.

TODO: describe the actual format here

### `[formatters.<name>]`

This section describes the integration between a single formatter and
`prjfmt`.

* `files`: A list of glob patterns used to select files. Usually this would be
    something like `[ "*.sh" ]` to select all the shell scripts. Sometimes,
    full filenames can be passed. Eg: `[ "Makefile" ]`.

* `command`: A list of arguments to execute the formatter. This will be
    composed with the `args` attribute during invocation. The first argument
    is the name of the executable to run.

* `args`: A list of extra arguments to add to the command. This is typically
    project-specific arguments.

## Use cases

### CLI usage

As a developer, I want to run `prjfmt` in any folder and it would
automatically format all of the code, configured for the project. I don't want
to remember what tool to use, or their magic incantation.

### Editor integration

Editors often want to be able to format a file, before it gets written to disk.

Ideally, the editor would pipe the code in, pass the filename, and get the
formatted code out. Eg: `cat ./my_file.sh | prjfmt --stdin my_file.sh >
formatted_file.sh`

### CI integration

We can assume that code lives in a source control.

For example a Git integration would look like this:

```sh
#!/usr/bin/env bash
set -euo pipefail

# Format all of the code
prjfmt

# Check that there are no changes in the code
if [[ -n "$(git status --porcelain -unormal)" ]]; then
  echo "Some code needs formatting! Please run \`prjfmt\`.
  git status -unormal
  exit 1
fi
echo "OK"
```

## Interfaces

### Formatter integration

Formatters can ship with a config file, that describes and simplifies the
integration between the formatter and `prjfmt`.

Whenever a new formatter is described in the project's `prjfmt.toml` config
file, `prjfmt` will look for that formatter config in
`$dir/share/prjfmt.d/<formatter>.toml` where `$dir` is each folder in
`XDG_DATA_DIRS`.

Example:

In the `prjfmt.toml` file of the project:
```toml
[formatters.ormolu]
args = [
  "--ghc-opt", "-XBangPatterns",
  "--ghc-opt", "-XPatternSynonyms",
]
```

Now assuming that ormolu ships with `$out/share/prjfmt.d/ormolu.toml` that
contains:
```toml
command = ["ormolu", "--mode", "inplace"]
files = ["*.hs"]
args = []
```

The project will first load `prjfmt.toml`, take note of the ormolu formatter,
then load the `ormolu.toml` file, and merge back the `prjfmt.toml` config for
that section back on top. The end-result would be the same as if the user has
this in their config:
```toml
[formatters.ormolu]
command = ["ormolu", "--mode", "inplace"]
files = ["*.hs"]
args = [
  "--ghc-opt", "-XBangPatterns",
  "--check-idempotence"
]
```
Note how the empty ormolu `args` got overwritten by the project `args`.

## Related projects

* [EditorConfig](https://editorconfig.org/): unifies file indentations
  configuration on a per-project basis.

## Contributing

All contributions are welcome! We try to keep the project simple and focused
so not everything will be accepted. Please open an issue to discuss before
working on a big item.

## License

MIT - (c) 2020 NumTide Ltd.
