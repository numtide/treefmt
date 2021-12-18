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
  cargoToml = with builtins; (fromTOML (readFile ./Cargo.toml));

  # What is used when invoking `nix run github:numtide/treefmt`
  treefmt = rustPackages.rustPlatform.buildRustPackage {
    inherit (cargoToml.package) name version;

    src = nixpkgs.lib.cleanSource ./.;

    # Those are being used in tests
    nativeBuildInputs = with nixpkgs; [
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
    ];

    shellHook = ''
      # Put the treefmt binary on the PATH when it's built
      export PATH=$PWD/target/debug:$PATH
    '';

    buildInputs = with nixpkgs; lib.optionals stdenv.isDarwin [ darwin.apple_sdk.frameworks.Security libiconv ];

    doCheck = true;

    cargoLock.lockFile = ./Cargo.lock;

    meta.description = "one CLI to format the code tree";
  };
in
{
  inherit treefmt;

  # A collection of packages for the project
  docs = nixpkgs.callPackage ./docs { };

  # Flake attributes
  defaultPackage = treefmt;
  devShell = treefmt;
}
