{
  description = "treefmt";
  # To update all inputs:
  # $ nix flake update --recreate-lock-file
  inputs.devshell.url = "github:numtide/devshell";
  inputs.flake-utils.url = "github:numtide/flake-utils";
  inputs.naersk.inputs.nixpkgs.follows = "nixpkgs";
  inputs.naersk.url = "github:nmattia/naersk";

  outputs = { self, nixpkgs, naersk, flake-utils, devshell }:
    (
      flake-utils.lib.eachSystem [ "x86_64-linux" "x86_64-darwin" ] (system:
        let
          pkgs = import nixpkgs {
            inherit system;
            # Makes the config pure as well. See <nixpkgs>/top-level/impure.nix:
            config = { };
            overlays = [
              naersk.overlay
              devshell.overlay
            ];
          };
        in
        {
          defaultPackage = pkgs.naersk.buildPackage {
            src = self;
          };

          devShell = pkgs.devshell.fromTOML ./devshell.toml;

          docs = pkgs.callPackage ./docs { };
        }
      )
    );
}
