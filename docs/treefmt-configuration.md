# Configuration format

`treefmt` depends on the `treefmt.toml` to map file extensions to actual code
formatters. That file is searched for recursively from the current folder and
up unless the `--config <path>` option is passed.

## `[formatter.<name>]`

This section describes the integration between a single formatter and
`treefmt`.

- `command`: A list of arguments to execute the formatter. This will be
  composed with the `options` attribute during invocation. The first argument
  is the name of the executable to run.

- `options`: A list of extra arguments to add to the command. This is typically
  project-specific arguments.

- `includes`: A list of glob patterns used to select files. Usually this would be
  something like `[ "*.sh" ]` to select all the shell scripts. Sometimes,
  full filenames can be passed. Eg: `[ "Makefile" ]`.

- `excludes`: A list of glob patterns to deny. If any of these patterns match,
  the file will be excluded.
