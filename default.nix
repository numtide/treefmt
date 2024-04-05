{ system ? builtins.currentSystem
, inputs ? import ./flake.lock.nix { }
, overlays ? [ (import inputs.rust-overlay) ]
, nixpkgs ? import inputs.nixpkgs {
    inherit system overlays;
    # Makes the config pure as well. See <nixpkgs>/top-level/impure.nix:
    config = { };
  }
, rustVersion ? nixpkgs.rust-bin.stable."1.65.0".default
, mkdocs-numtide ? import inputs.mkdocs-numtide { pkgs = nixpkgs; }
}:
let
  lib = nixpkgs.lib;

  # Override license so that we can build terraform without
  # having to re-import nixpkgs.
  terraform' = nixpkgs.terraform.overrideAttrs (old: { meta = { }; });

  rustVersionExtended = rustVersion.override {
    # include source for IDE's and other tools that resolve the source automatically via
    # $(rustc --print sysroot)/lib/rustlib/src/rust
    extensions = [ "rust-src" "rustfmt" ];
  };

  rustPlatform = nixpkgs.makeRustPlatform {
    cargo = rustVersionExtended;
    rustc = rustVersionExtended;
  };

  cargoToml = with builtins; (fromTOML (readFile ./Cargo.toml));

  # Use the Nix module system to validate the treefmt config file format.
  evalModule = config:
    throw "treefmt.evalModule has been moved to https://github.com/numtide/treefmt-nix";

  # What is used when invoking `nix run github:numtide/treefmt`
  treefmt = rustPlatform.buildRustPackage {
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

    meta.description = "one CLI to format your repo";

    passthru.withConfig = config:
      throw "treefmt.withConfig has been moved to https://github.com/numtide/treefmt-nix";
  };

  # Add all the dependencies of treefmt, plus more build tools
  devShell = treefmt.overrideAttrs (prev: {
    shellHook = ''
      # Put the treefmt binary on the PATH when it's built
      export PATH=$PWD/target/debug:$PATH

      # Export location of llvm tools for use in code coverage
      # see https://discourse.nixos.org/t/llvm-profdata-grcov-and-rust-code-coverage/19849/2 for background
      export LLVM_PATH=$(dirname "$(type -p llvm-profdata)")
    '';

    nativeBuildInputs = prev.nativeBuildInputs ++ (with nixpkgs; [
      # Build tools
      grcov
      rust-analyzer
      rustc.llvmPackages.llvm
      just

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
      shellcheck
      shfmt
      terraform'

      # Docs
      mkdocs-numtide
    ]);
  });
in
{
  inherit treefmt devShell evalModule;

  # reduce a bit of repetition
  inherit (treefmt.passthru) withConfig;

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
