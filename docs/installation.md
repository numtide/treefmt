# Installation

You can install `treefmt` with two option below. The best option is to download the release binary as you will get the stable version of `treefmt`.

## Download release binary

Download the stable version of `treefmt` from [release binary](https://github.com/numtide/treefmt/releases).

## Building from source

### Non-Nix User

Install `rust` using [`rustup`] by following the instruction.

To try the project, run:

```
$ cargo run -- --help
```

If you want to build the project, run:

```
$ cargo build
```

and find the `treefmt` binary in the `target` folder.

[`rustup`]: https://rustup.rs/

### Nix User

#### Non-flake user

This repository can be used using plain `nix-build` or `nix-shell`. To build
the package, just run:

```
$ nix-build -A treefmt
```

#### Nix-flake user

If you want to use this repo with `flakes` feature, please enable it using the following method:

**Linux and Windows Subsystem Linux 2 (WSL2)**

Install Nix as instructed above. Next, install `nixUnstable` by running the following code:

```
nix-env -iA nixpkgs.nixFlakes
```

Lastly, open your `~/.config/nix/nix.conf` or `/etc/nix/nix.conf` file and add:

```
experimental-features = nix-command flakes
```

**NixOS**

Add the following code into your `configuration.nix`:

```
{ pkgs, ... }: {
  nix = {
    package = pkgs.nixFlakes;
      extraOptions = ''
        experimental-features = nix-command flakes
      '';
  };
}
```

And finally, run the following command:

```
$ nix build
```

The `treefmt` binary will be available in the `result` folder.

Alternatively, you can run:

```
$ nix run . -- --help
```

From the root of this project.
