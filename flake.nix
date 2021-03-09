{
  description = "treefmt";
  # To update all inputs:
  # $ nix flake update --recreate-lock-file
  inputs.devshell.url = "github:numtide/devshell";
  inputs.flake-utils.url = "github:numtide/flake-utils";
  inputs.naersk.inputs.nixpkgs.follows = "nixpkgs";
  inputs.naersk.url = "github:nmattia/naersk";
  inputs.rust-overlay.url = "github:oxalica/rust-overlay";
  inputs.rust-overlay.inputs.nixpkgs.follows = "nixpkgs";

  outputs = { self, nixpkgs, naersk, flake-utils, devshell, rust-overlay }:
    (
      flake-utils.lib.eachSystem [ "x86_64-linux" "x86_64-darwin" ] (system:
        let
          pkgs = import nixpkgs {
            inherit system;
            # Makes the config pure as well. See <nixpkgs>/top-level/impure.nix:
            config = { };
            overlays = [
              rust-overlay.overlay
              naersk.overlay
              devshell.overlay
            ];
          };

          # TODO: filter the source to minimize rebuilds. Cargo.* src/
          src = self;

          treefmt = pkgs.naersk.buildPackage {
            inherit src;
          };

          treefmt-cross = targetSystem:
            let
              # TODO: pin nightly version
              # We use nightly because naersk needs nighly for cargo
              rust = pkgs.rust-bin.nightly.latest.rust.override {
                extensions = [
                  "rust-analysis"
                  "rust-std"
                  "rust-src"
                ];
                targets = [ targetSystem ];
              };
            in
            (pkgs.naersk.override {
              rustc = rust;
              cargo = rust;
            }).buildPackage {
              inherit src;
              CARGO_BUILD_TARGET = targetSystem;
              # Rewrite /nix entries to make the target leaner
              remapPathPrefix = true;
            };
        in
        {
          # What is used when invoking `nix run github:numtide/treefmt`
          defaultPackage = treefmt;

          # A collection of packages for the project
          legacyPackages = {
            inherit treefmt;
            docs = pkgs.callPackage ./docs { };

            # Cross for releases
            release = {
              linux = treefmt-cross "x86_64-unknown-linux-musl";
              macos = treefmt-cross "x86_64-apple-darwin";
              windows = treefmt-cross "x86_64-pc-windows-gnu";
            };
          };

          # The development environment
          devShell = pkgs.devshell.fromTOML ./devshell.toml;
        }
      )
    );
}
