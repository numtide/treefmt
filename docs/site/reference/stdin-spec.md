---
outline: deep
---

# Stdin Specification

Formatters _MAY_ also implement the Stdin Specification, which allows
formatting _virtual files_ passed via stdin.

A formatter **MUST** implement the Stdin Specification if its formatting behavior
can depend on the path of the file being formatted.

## Terms

- _virtual file_: Conceptually, represents some file the formatter is supposed to
  treat "as if" it existed on disk. Concretely, a _virtual file_ is a buffer (passed via stdin)
  and an _advisory path_ (passed as a CLI option).
- _advisory path_: Represents the path of some _virtual file_. Called
  "advisory" because it _MAY_ be passed to a formatter, and the formatter _MAY_
  alter its behavior based on the given advisory path.

## Rules

In order for the formatter to comply with this spec, it **MUST** implement the
vanilla [Formatter Specification](/reference/formatter-spec), and additionally
satisfy the following:

### 1. Command line option

The formatter's CLI **MUST** have some option to activate [Stdin mode](#2-stdin-mode).

The CLI _SHOULD_ accept an _advisory path_ to make the formatter pretend stdin comes
from this file.

The formatter _MAY_ alter its behavior based on the given
_advisory path_ `<path>`. For example, if there are different formatting
rules in different directories, or for use in error messages. If the
formatter's behavior doesn't depend on the given `<path>`, it's ok to ignore
it.

The CLI option _SHOULD_ be called `--stdin-filepath`:

```console
$ echo "{}" | nixfmt --stdin-filepath path/to/file.nix
```

However, other options are OK, such as `-path`:

```console
$ echo 'print( "hello")' | buildifier -path foo.bzl
```

It's OK if the formatter does not accept an _advisory path_ (which implies
the formatter's behavior does not depend on file paths, which means the Stdin
Specification is optional).

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
