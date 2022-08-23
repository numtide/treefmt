{
  description = "treefmt";
  # To update all inputs:
  # $ nix flake update --recreate-lock-file
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

  outputs = { self, nixpkgs, flake-parts }@inputs:
    flake-parts.lib.mkFlake { inherit self; } {
      systems = nixpkgs.lib.systems.flakeExposed;
      perSystem = { system, pkgs, ... }:
        let
          packages = import ./. {
            inherit system;
            inputs = null;
            nixpkgs = pkgs;
          };
        in
        {
          inherit packages;

          devShells.default = packages.devShell;
        };
    };
}
