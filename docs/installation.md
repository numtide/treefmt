---
outline: deep
---

# Installation

There are two options to install `treefmt`:

1. Downloading the latest binary
2. Compiling and building it from source.

## Installing with a binary file

You can find the list of the latest binaries [here](https://github.com/numtide/treefmt-go/releases).

## Building from source

There are several ways to build `treefmt` from source. Your choice will depend on whether you're a [nix](https://github.com/NixOS/nix) user or
not.

### Non-Nix User

To try the project without building it, run:

```
$ go run main.go --help
```

The command will output the manual. You can run the tool in this manner with any other flag or option to format your
project.

To build a binary, you need to have `go 1.22` installed. You can find instructions [here](https://go.dev/doc/install).

Now, if you want to build the project, switch to the project root folder and run:

```
$ go build
```

After the successful execution of the build command, you will find the `treefmt` binary in the project root folder.

### Nix User

If you're using both `treefmt` and `nix`, you can go for [`treefmt-nix`](https://github.com/numtide/treefmt-nix), a wrapper that makes installation and
configuration of `treefmt` with `nix` easier.

**Non-flake user**

Here you also have two options: you can install `treefmt` with plain nix-build , or with nix-shell.

To build the package with nix-build, just run:

```
$ nix-build -A treefmt
```

**Nix-flake user**

If you want to use this repository with flakes, please enable the flakes feature first. To run the project with flakes without building it, you can execute the following command in the root folder:

```
$ nix run . -- --help
```

To build the project, run the following command in the root folder:

```
$ nix build
```

The `treefmt` binary will be available in the result folder.
