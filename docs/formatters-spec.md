# Formatter specification

In order to keep the design of `treefmt` simple, we support only formatters which adhere to a certain standard. This document outlines this standard. If the formatter you would like to use doesn't comply with the rules, you should create a wrapper script that transforms the usage to match the specification.

## Command-line interface

In order to be integrated to `treefmt`'s workflow, the formatter's CLI must adhere to the following specification:

```
<command> [options] [...<files>]
```

Where:

- `<command>` is the name of the formatting tool.
- `[options]` is any number of flags and options that the formatter accepts.
- `[...<files>]` is one or more files given to the formatter for processing.

Example:

```
$ rustfmt --edition 2018 src/main.rs src/lib.rs
```

Whenever a formatter is invoked with a list of files, it should processes only the specified files. Files that are not passed should never be formatted.

If, and only if, a file has changed, the formatter will write the new content in place of the original one.

## Other requirements

You must ensure that the formatter you're planning to use:

- **Preserves code validity:** This is a strong contract; the syntax and semantics must never be broken by the formatter.
- **Is idempotent:** if it is run twice on a file, the file should not change on the second invocation.

`treefmt` guarantees that the formatter won't traverse the file system if a list of files is passed to it.