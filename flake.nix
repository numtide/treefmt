{
  description = "treefmt";
  # To update all inputs:
  # $ nix flake update --recreate-lock-file
  inputs.devshell.url = "github:numtide/devshell";
  inputs.flake-utils.url = "github:numtide/flake-utils";
  inputs.naersk.inputs.nixpkgs.follows = "nixpkgs";
  inputs.naersk.url = "github:nmattia/naersk";
  inputs.rust-overlay.url = "github:oxalica/rust-overlay";

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

          rust = pkgs.rust-bin.stable.latest.rust.override {
            extensions = [
              "clippy-preview"
              "rustfmt-preview"
              "rust-analysis"
              "rust-std"
              "rust-src"
            ];
            targets = [
              "wasm32-unknown-unknown"
              "x86_64-unknown-linux-musl"
            ];
          };

          treefmt = (pkgs.naersk.override {
            rustc = rust;
          }).buildPackage {
            src = self;
            CARGO_BUILD_TARGET = "x86_64-unknown-linux-musl";
          };
        in
        {
          # What is used when invoking `nix run github:numtide/treefmt`
          defaultPackage = treefmt;

          # A collection of packages for the project
          packages = {
            inherit treefmt;
            docs = pkgs.callPackage ./docs { };
          };

          # The development environment
          devShell = pkgs.devshell.fromTOML ./devshell.toml;
        }
      )
    );
}
