# FAQ

## How does treefmt function?

`Treefmt` traverses all your project's folders, maps files to specific code formatters, and formats the code accordingly. Other tools also traverse the filesystem, but not necessarily starting from the root of the project.

Contrary to other formatters, `treefmt` doesn't preview the changes before writing them to a file. If you want to view the changes, you can always check the diff in your version control (we assume that your project is checked into a version control system). You can also rely on version control if errors were introduced into your code as a result of disruptions in the formatter's work.

## How is the cache organized?

At this moment, the cache is represented by a flat TOML file where file paths are mapped to `mtimes`. The file is located in:

```
~/.cache/treefmt/<hash-of-the-treefmt.toml-path>.toml
```

However, we are planning to move the hash file to the destination project's root directory.

At the end of each tool run, the cache file gets overwritten with the last formatting time entries. In this way, we can can compare the last change time of the file to the last formatting time, and figure out which files need re-formatting.
