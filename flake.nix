{
  description = "treefmt";
  # To update all inputs:
  # $ nix flake update --recreate-lock-file
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  inputs.flake-parts.url = "github:hercules-ci/flake-parts";
  inputs.flake-parts.inputs.nixpkgs.follows = "nixpkgs";

  inputs.rust-overlay.url = "github:oxalica/rust-overlay";
  inputs.rust-overlay.inputs.nixpkgs.follows = "nixpkgs";

  outputs = { self, nixpkgs, flake-parts, rust-overlay }@inputs:
    flake-parts.lib.mkFlake { inherit self; } {
      systems = nixpkgs.lib.systems.flakeExposed;
      perSystem = { system, pkgs, ... }:
        let
          packages = import ./. {
            inherit system;
          };
        in
        {
          # This contains a mix of packages, modules, ...
          legacyPackages = packages;

          devShells.default = packages.devShell;

          # In Nix 2.8 you can run `nix fmt` to format this whole repo.
          #
          # Because we load the treefmt.toml and don't define links to the
          # packages in Nix, the formatter has to run inside of `nix develop`
          # to have the various tools on the PATH.
          #
          # It also assumes that the project root has a flake.nix (override this by setting `projectRootFile`).
          formatter = packages.treefmt.withConfig {
            settings = nixpkgs.lib.importTOML ./treefmt.toml;
            projectRootFile = "flake.nix";
          };
        };
    };
}
