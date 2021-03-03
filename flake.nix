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
            crossOverlays = [
              (import "${nixpkgs}/pkgs/top-level/static.nix")
            ];
            crossSystem = {
                isStatic = true;
                config = "x86_64-unknown-linux-musl";
            };
            overlays = [
              naersk.overlay
              devshell.overlay
            ];
          };

          treefmt = (pkgs.naersk.override {
              rustc = pkgs.rustPackages.rustc;
              cargo = pkgs.rustPackages.cargo;
            }).buildPackage {
              src = self;
              remapPathPrefix = true;
              nativeBuildInputs = with pkgs.buildPackages; [ pkgconfig ];
              PKG_CONFIG_ALLOW_CROSS = "true";
              PKG_CONFIG_ALL_STATIC = "true";
              LIBZ_SYS_STATIC = "1";
              CARGO_BUILD_TARGET = "x86_64-unknown-linux-musl";
              CARGO_TARGET_X86_64_UNKNOWN_LINUX_MUSL_LINKER = "${pkgs.buildPackages.llvmPackages_10.lld}/bin/lld";
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
