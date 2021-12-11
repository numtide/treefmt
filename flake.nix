{
  description = "treefmt";
  # To update all inputs:
  # $ nix flake update --recreate-lock-file
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  inputs.flake-utils.url = "github:numtide/flake-utils";

  outputs = { self, nixpkgs, flake-utils }@inputs:
    flake-utils.lib.eachSystem [ "x86_64-linux" "x86_64-darwin" ] (system:
      let
        pkgs = import self {
          inherit system;
          inputs = null;
          nixpkgs = nixpkgs.legacyPackages.${system};
        };
      in
      {
        defaultPackage = pkgs.defaultPackage;
        legacyPackages = pkgs;
        devShell = pkgs.devShell;
      }
    );
}
