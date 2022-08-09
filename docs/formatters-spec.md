# Formatter spec

In order to keep the design of treefmt simple, we want all of the formatters
to adhere to a unified specification. This document outlines that spec.

## Command-line interface

A formatter MUST adhere to the following interface:

```
<command> [options] [...<files>]
```

Where

- `<command>` is the name of the formatter.
- `[options]` is any number of flags and options that the formatter wants to
  provide.
- `[...<files>]` is one or more files that the formatter should process.

Whenever the program is invoked with the list of files, it MUST only process all the files that are passed and format them. Files that are not passed should never be formatted.

If, and only if, a file format has changed, the formatter MUST write the new
content in place of the original file.

### Example

```console
$ rustfmt --edition 2018 src/main.rs src/lib.rs
```

## Formatting details

A formatted file MUST be valid. This is a strong contract; the syntax or
semantic must never be broken by the formatter.

A formatter SHOULD be idempotent. Meaning that if the formatter it run
twice on a file, the file should not change on the second invocation.

The formatter MUST NOT do file traversal when a list of files is passed to it.
This is the responsibility of `treefmt`.

## Design notes

We assume that the code is stored in source control, which is why it's fine to
write the new content in place.
