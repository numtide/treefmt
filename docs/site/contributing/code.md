# Code

## Pre-requisites

You will need to have the following installed:

- [Nix]
- [Direnv]

!!! important

    We use a [Flake]-based workflow. You can certainly develop for `treefmt` without Flakes and leverage
    much of what is listed below, but it is left up to the reader to determine how to make that work.

## Go development

The default [devshell] provides all the necessary tooling and utilities for working on `treefmt`.

```nix title="nix/devshells/default.nix"
--8<-- "nix/devshells/default.nix"
```

[Direnv] should load this by default when entering the root of the repository.

For the most part, you _should_ be able to develop normally as you would any other Go program.

!!! important

    When you have completed making any changes and have tested it as you would any other Go program, it is important
    to ensure it works when run as a Nix package.

    This can be done with `nix run .# -- <args>`, which will build the Nix derivation and execute the resultant
    `treefmt` binary.

## Formatting

We use the latest released version of [treefmt] and [treefmt-nix] to format the repository by running `nix fmt` from
the root directory.

```nix title="nix/formatter.nix"
--8<-- "nix/formatter.nix"
```

## Checks

Running `nix flake check` will build all the devshells and Nix packages, as well as check the formatting with [treefmt]
and any other [Flake checks](https://github.com/NixOS/nix/blob/master/src/nix/flake-check.md) that have been configured.

## Documentation

When making changes, it is **important** to add or update any relevant sections in the documentation within the same
pull request.

For more information see the [next section](./docs.md).

[Nix]: https://nixos.org
[Flake]: https://wiki.nixos.org/wiki/Flakes
[Nix derivation]: https://nix.dev/manual/nix/2.18/language/derivations
[Direnv]: https://direnv.net
[devshell]: https://nix.dev/tutorials/first-steps/declarative-shell.html
[treefmt]: https://treefmt.com
[treefmt-nix]: https://github.com/numtide/treefmt-nix
