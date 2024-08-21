---
outline: deep
---

# Install Treefmt

There are two ways to install `treefmt`:

1. Download the latest binary
2. Compile and build from source.

## Download a binary file

You can download the latest `treefmt` binaries [here](https://github.com/numtide/treefmt/releases).

## Build from source

There are several ways to build `treefmt` from source. Your choice will depend on whether you're a [nix](https://github.com/NixOS/nix) user or
not.

### Non-Nix User

To try the project without building it, run the following from the project root folder:

```
$ go run main.go --help
```

This command will output the manual. You can run the tool in this manner with any other flag or option to format your
project.

To build a binary, you need to have `go 1.22` installed. You can find instructions on how to install a `go` compiler
[here](https://go.dev/doc/install).

To build the project, run the following:

```
$ go build
```

After the build command exits successfully, you will find the `treefmt` binary in the project root folder.

### Nix User

If you're using both `treefmt` and `nix`, you can make use of [`treefmt-nix`](https://github.com/numtide/treefmt-nix), a wrapper that makes installing and
configuring `treefmt` with `nix` easier.

**Non-flake user**

Here you also have two options: you can install `treefmt` with plain `nix-build`, or with `nix-shell`.

To build the package with `nix-build`, run the following:

```
$ nix-build -A packages.x86_64-linux.treefmt
```

> note: substitute `x86_64-linux` for the target system you with to build for

**Nix-flake user**

If you want to use this repository with flakes, first ensure you have [flakes enabled](https://wiki.nixos.org/wiki/Flakes).
You can then execute the following command in the project root folder:

```
$ nix run . -- --help
```

To build the project, run the following command in the project root folder:

```
$ nix build
```

The `treefmt` binary will be available in the `result` folder.
