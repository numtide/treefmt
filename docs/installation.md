# Installation

There are two options to install `treefmt`: by downloading the latest binary, or by compiling and building the tool from source.

## Installing with a binary file

You can find the list of the latest binaries [here](https://github.com/numtide/treefmt/releases).

## Building from source

There are several ways to build `treefmt` from source. Your choice will depend on whether you're a [nix](https://github.com/NixOS/nix) user.

### Non-Nix User

To try the project without building it, run:

```
$ cargo run -- --help
```

The command will output the manual. You can run the tool in this manner with any other flag or option to format your project.

To build a binary, you need to have rust installed. You can install it with [rustup](https://rustup.rs/). Now, if you want to build the project, switch to the project root folder and run:

```
$ cargo build
```

After the successful execution of the cargo build command, you will find the `treefmt` binary in the target folder.

### Nix User

[Nix](https://github.com/NixOS/nix) is a package manager foundational for NixOS. You can use it in NixOS and in any other OS equally.

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