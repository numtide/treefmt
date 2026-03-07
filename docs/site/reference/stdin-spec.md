---
outline: deep
---

# Stdin Specification

Formatters **MAY** also implement the Stdin Specification, which allows
formatting "virtual files" passed via stdin.

A formatter **MUST** implement the Stdin Specification if its formatting behavior
can depend on the path of the file being formatted.

## Rules

In order for the formatter to comply with this spec, it **MUST** implement the
vanilla [Formatter Specification](/reference/formatter-spec), and additionally
satisfy the following:

### 1. Command line option

The formatter's CLI **MUST** have some cli option to activate stdin mode. The
CLI _SHOULD_ accept an advisory path to make the formatter pretend stdin comes
from this file. The formatter _MAY_ alter its behavior based on the given
"advisory path" `<path>`. For example, if a there are different formatting
rules in different directories, or just to include in error messages. If the
formatter's behavior doesn't depend on the given `<path>`, it's ok to ignore
it.

The CLI option _SHOULD_ be called `--stdin-filepath`:

```console
$ echo "{}" | nixfmt --stdin-filepath path/to/file.nix
```

However, other options are OK, such as `-path`:

```console
$ echo 'print( "hello")' | buildifier -path foo.py
```

It's OK if the formatter doesn't accept an advisory path (which implies
the formatter's behavior does not depend on file paths, which means the stdin
specification is optional).

```console
$ echo 'print( "hello")' | ruff format -
```

### 2. Stdin mode

When in stdin mode, the formatter:

1. **MUST** print the formatted file to stdout.
2. **MUST NOT** attempt to read the file on the filesystem. Instead, it
   **MUST** read from stdin.
3. **MUST NOT** write to the given path on the filesytem. It _MAY_ write to
   temporary files elsewhere on disk, but _SHOULD_ clean them up when done.
