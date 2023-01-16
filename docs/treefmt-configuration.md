# Configuration

`treefmt` can only be run in the presence of `treefmt.toml` where files are mapped to specific code formatters.

Usually the config file sits in the project root folder. If you're running `treefmt` in one of the project's folders, then `treefmt` will look for the config in the parent folders up until the project's root. However, you can place the config anywhere in your project's file tree and specify the path in the the ---config-file flag.

The typical section of `treefmt.toml` looks like this:

```
[formatter.<name>]
command = "<formatter-command>"
options = ["<formatter-option-1>"...]
includes = ["<regular-expression>"]
```

...where name is just an identifier.

```
[formatter.elm]
command = "elm-format"
options = ["--yes"]
includes = ["*.elm"]
```

Make sure you installed all the formatters specified in the config before running `treefmt`. If you don't want to install all formatters, you can still run `treefmt` by specifying the flag `--allow-missing-formatter`. This will make the program not error out if the needed formatter is missing.

## Configuration format

### `[formatter.<name>]`

This section describes the integration between a single formatter and treefmt. "Name" here is a unique ID of your formatter in the config file. It doesn't have to match the formatter name.

- `command`: A list of arguments to be executed. This will be concatenated with the options attribute during invocation. The first argument is the name of the executable to run.
- `options`: A list of extra arguments to add to the command. These are typically project-specific arguments.
- `includes`: A list of glob patterns to match file names, including extensions and paths, used to select specific files for formatting. Typically, only file extensions are specified to pick all files written in a specific language. For instance,[`"*.sh"`] selects shell script files. But sometimes, you may need to specify a full file name, like [`"Makefile"`], or a pattern picking files in a specific folder, like [`"/home/user/project/*"`].

- `excludes`: A list of glob  patterns to exclude from formatting. If any of these patterns match, the file will be excluded from formatting by a particular formatter.

### `[global]`

This section describes the configuration properties that apply to every formatter.

- `excludes`: A list of glob patterns to deny. If any of these patterns match, the file won't be formatted. This list is appended to the individual formatter's excludes lists.