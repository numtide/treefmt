{ system ? builtins.currentSystem
, inputs ? import ./flake.lock.nix { }
, nixpkgs ? import inputs.nixpkgs {
    inherit system;
    # Makes the config pure as well. See <nixpkgs>/top-level/impure.nix:
    config = { };
    overlays = [ ];
  }
, rustPackages ? nixpkgs.rustPackages
}:
let
  lib = nixpkgs.lib;

  cargoToml = with builtins; (fromTOML (readFile ./Cargo.toml));

  # Use the Nix module system to validate the treefmt config file format.
  evalModule = config:
    lib.evalModules {
      modules = [
        {
          _module.args = { inherit nixpkgs lib treefmt; };
        }
        ./module-options.nix
        config
      ];
    };

  # What is used when invoking `nix run github:numtide/treefmt`
  treefmt = rustPackages.rustPlatform.buildRustPackage {
    inherit (cargoToml.package) name version;

    src = builtins.path {
      path = ./.;
      filter = name: type:
        name == toString ./Cargo.toml
        || name == toString ./Cargo.lock
        || lib.hasPrefix (toString ./src) name
        || lib.hasPrefix (toString ./benches) name
      ;
    };

    buildInputs = with nixpkgs; lib.optionals stdenv.isDarwin [ darwin.apple_sdk.frameworks.Security libiconv ];

    doCheck = true;

    cargoLock.lockFile = ./Cargo.lock;

    meta.description = "one CLI to format the code tree";

    passthru.withConfig = config:
      let
        mod = evalModule config;
      in
      mod.config.build.wrapper;
  };

  # Add all the dependencies of treefmt, plus more build tools
  devShell = treefmt.overrideAttrs (prev: {
    shellHook = ''
      # Put the treefmt binary on the PATH when it's built
      export PATH=$PWD/target/debug:$PATH
    '';

    nativeBuildInputs = prev.nativeBuildInputs ++ (with nixpkgs; [
      # Build tools
      rustPackages.clippy
      rust-analyzer

      # Code formatters
      elmPackages.elm-format
      go
      haskellPackages.cabal-fmt
      haskellPackages.ormolu
      mdsh
      nixpkgs-fmt
      nodePackages.prettier
      python3.pkgs.black
      rufo
      rustPackages.rustfmt
      shellcheck
      shfmt
      terraform

      mdbook
    ]);
  });
in
{
  inherit treefmt devShell evalModule;

  # module that generates and wraps the treefmt config with Nix
  module = ./module-options.nix;

  # reduce a bit of repetition
  inherit (treefmt.passthru) withConfig;

  # A collection of packages for the project
  docs = nixpkgs.callPackage ./docs { };

  # Flake attributes
  default = treefmt;

  # Test that no HOME is needed when --no-cache is passed
  treefmt-no-cache-no-home = nixpkgs.runCommandLocal "format"
    {
      buildInputs = [ treefmt ];
    }
    ''
      cat <<CONFIG > treefmt.toml
      [formatter.nix]
      command = "${nixpkgs.nixpkgs-fmt}/bin/nixpkgs-fmt"
      includes = ["*.nix"]
      CONFIG
      # uncommenting this makes it work fine
      # export HOME=$TMP
      treefmt --no-cache --fail-on-change -C ./.
      touch $out
    '';
}
