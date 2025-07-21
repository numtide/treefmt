---
outline: deep
---

# Formatter Specification

To keep the design of `treefmt` simple, we only support formatters that adhere to a certain standard.
This document outlines that standard.

In this design, we rely on `treefmt` to do the tree traversal, and only invoke
the formatter on the selected files.

!!! note

    If the formatter you would like to use doesn't comply with the rules, it's often possible to create a wrapper script
    that transforms the usage to match the specification.

## Rules

In order for the formatter to comply with this spec, it **MUST** satisfy the following:

### 1. Files passed as arguments

The formatter's CLI must be of the form:

```
<command> [options] [...<files>]
```

Where:

- `<command>` is the name of the formatter executable.
- `[options]` is any number of flags and options that the formatter accepts.
- `[...<files>]` is one or more files given to the formatter for processing.

Example:

```
$ rustfmt --edition 2018 src/main.rs src/lib.rs
```

!!! note

    It **MUST** process the specified files. For example, it **MUST NOT** ignore files because they are not tracked by a VCS.

    It _SHOULD_ processes only the specified files. Files that are not passed _SHOULD_ never be formatted.

### 2. Write to changed files

Whenever there is a change to the code formatting, the formatter **MUST** write those changes back to the
original location.

If there are no changes to the original file, the formatter **MUST NOT** write to the original location.

### 3. Exit nonzero on error

When something goes wrong when formatting (for example, the source code is
syntactically invalid), the formatter **MUST** exit nonzero.

The formatter _SHOULD_ print useful information about what went wrong to
stderr.

### 4. Idempotent

The formatter _SHOULD_ be idempotent. Meaning that it produces stable
outputs.

### 5. Reliable

We expect the formatter to be reliable and not break the semantics of the formatted files.
