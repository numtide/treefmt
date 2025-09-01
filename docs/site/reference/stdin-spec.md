---
outline: deep
---

# Stdin Specification

Formatters **MAY** also implement the Stdin Specification, which allows
formatting "virtual files" passed via stdin.

A formatter **MUST** implement the Stdin Specification if its formatting behavior
can depend on the name of the file being formatted.

## Rules

In order for the formatter to comply with this spec, it **MUST** implement the
vanilla [Formatter Specification](/reference/formatter-spec), and additionally
satisfy the following:

### 1. `--stdin-filepath` flag

The formatter's CLI **MUST** be of the form:

```
<command> [options] [--stdin-filepath <path>]
```

Where:

- `<command>` is the name of the formatting tool.
- `[options]` are any number of flags and options that the formatter accepts.
- `--stdin-filepath <path>` is an optional flag that puts the formatter in
  "stdin mode". In stdin mode, the formatter reads file contents from stdin
  rather than the filesystem.
    - The formatter _MAY_ alter its behavior based on the given `<path>`. For
      example, if a there are different formatting rules in different
      directories. If the formatter's behavior doesn't depend on the given
      `<path>`, it's ok to ignore it.
    - The formatter _MAY_ understand `--stdin-filepath=<path>` as well, but **MUST**
      understand the space separated variant.

Example:

```
$ echo "{}" | nixfmt --stdin-filepath path/to/file.nix
```

### 2. Print to stdout, do not assume file is present on filesystem

When in stdin mode, the formatter:

1. **MUST** print the formatted file to stdout.
2. **MUST NOT** attempt to read the file on the filesystem. Instead, it
   **MUST** read from stdin.
3. **MUST NOT** write to the given path on the filesytem. It _MAY_ write to
   temporary files elsewhere on disk, but _SHOULD_ clean them up when done.
