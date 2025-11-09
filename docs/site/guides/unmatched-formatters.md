# Handling Unmatched Files

By default, treefmt lists all files that aren't matched by any formatter:

```console
$ treefmt
WARN no formatter for path: .gitignore
WARN no formatter for path: LICENSE
WARN no formatter for path: README.md
WARN no formatter for path: go.mod
WARN no formatter for path: go.sum
WARN no formatter for path: build/build.go
# ...
```

This helps you decide whether to add formatters for specific files or ignore them entirely.

## Customizing Notifications

### Reducing Log Verbosity

If you find the unmatched file warnings too noisy, you can lower the logging level in your config:

`treefmt.toml`:

```toml
on-unmatched = "debug"
```

To later find out what files are unmatched, you can override this setting via the command line:

```console
$ treefmt --on-unmatched warn
```

### Enforcing Strict Matching

Another stricter policy approach is to fail the run if any unmatched files are found.
This can be paired with `excludes` and/or `reject` lists to ignore specific files:

`treefmt.toml`:

```toml
# Fail if any unmatched files are found
on-unmatched = "fatal"

# List files to explicitly ignore
excludes = [
  "LICENCE",
  "go.sum",
]

# List of templates for rejecting files.  If any template evaluates to `false`
# for a given file, that file will be ignored.
reject = [
  "{{ .HasExt }}",                      # reject files with extensions
  "{{ not .IsExecutable }}",            # reject files that are not executable
  "{{ fnmatch `donotwant` .Shebang }}", # reject files with "donotwant" in the shebang
]
```
