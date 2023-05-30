# Formatter specification

In order to keep the design of `treefmt` simple, we support only formatters which adhere to a certain standard. This document outlines this standard. If the formatter you would like to use doesn't comply with the rules, it's often possible to create a wrapper script that transforms the usage to match the specification.

In this design, we rely on `treefmt` to do the tree traversal, and only invoke
the code formatter on the selected files.

## Rules

In order for the formatter to comply to this spec, it MUST follow the
following rules:

### 1. Files passed as arguments

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

It SHOULD processes only the specified files. Files that are not passed SHOULD never be formatted.

### 2. Write to changed files

Whenever there is a change to the code formatting, the code formatter MUST
write to the changes back to the original location.

If there is no changes to the original file, the formatter MUST NOT write to
the original location.

### 3. Idempotent

The code formatter SHOULD be indempotent. Meaning that it produces stable
outputs.

### 4. Reliable

We expect the formatter to be reliable and not break the semantic of the
formatted files.
