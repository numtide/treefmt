{ system ? builtins.currentSystem
, inputs ? import ./flake.lock.nix { }
}:
let
  nixpkgs = import inputs.nixpkgs {
    inherit system;
    # Makes the config pure as well. See <nixpkgs>/top-level/impure.nix:
    config = { };
  };

  devshell = import inputs.devshell {
    inherit system;
    pkgs = nixpkgs;
  };

  cargoToml = with builtins; (fromTOML (readFile ./Cargo.toml));
in
{
  # What is used when invoking `nix run github:numtide/treefmt`
  treefmt = nixpkgs.pkgs.rustPlatform.buildRustPackage
    {
      src = nixpkgs.lib.cleanSource ./.;
      inherit (cargoToml.package) name version;
      cargoLock.lockFile = ./Cargo.lock;
    } // { meta.description = "one CLI to format the code tree"; };

  # A collection of packages for the project
  docs = nixpkgs.callPackage ./docs { };

  # The development environment
  devShell = devshell.fromTOML ./devshell.toml;
}
