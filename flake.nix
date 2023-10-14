{
  description = "treefmt";
  # To update all inputs:
  # $ nix flake update --recreate-lock-file

  inputs.systems.url = "github:nix-systems/default";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  inputs.flake-parts.url = "github:hercules-ci/flake-parts";
  inputs.flake-parts.inputs.nixpkgs-lib.follows = "nixpkgs";

  inputs.rust-overlay.url = "github:oxalica/rust-overlay";
  inputs.rust-overlay.inputs.nixpkgs.follows = "nixpkgs";

  inputs.mkdocs-numtide.url = "github:numtide/mkdocs-numtide";
  inputs.mkdocs-numtide.inputs.nixpkgs.follows = "nixpkgs";

  outputs = { self, nixpkgs, flake-parts, mkdocs-numtide, systems, ... }@inputs:
    flake-parts.lib.mkFlake { inherit self; } {
      systems = import systems;
      perSystem = { system, self', lib, pkgs, ... }:
        let
          packages = import ./. {
            inherit system;
            mkdocs-numtide = mkdocs-numtide.packages.${system}.default;
          };
        in
        {
          # This contains a mix of packages, modules, ...
          legacyPackages = packages;

          # Allow `nix run github:numtide/treefmt`.
          packages.default = packages.treefmt;

          packages.docs = mkdocs-numtide.lib.${system}.mkDocs {
            name = "treefmt-docs";
            src = ./.;
          };

          checks =
            let
              packages = lib.mapAttrs' (n: lib.nameValuePair "package-${n}") self'.packages;
              devShells = lib.mapAttrs' (n: lib.nameValuePair "devShell-${n}") self'.devShells;
            in
            packages // devShells;

          devShells.default = packages.devShell;
        };
    };
}
