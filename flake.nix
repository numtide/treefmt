{
  description = "treefmt";
  # To update all inputs:
  # $ nix flake update --recreate-lock-file
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  inputs.flake-utils.url = "github:numtide/flake-utils";

  outputs = { self, nixpkgs, flake-utils }@inputs:
    flake-utils.lib.eachDefaultSystem (system:
      let
        nixpkgs' = nixpkgs.legacyPackages.${system};
        pkgs = import self {
          inherit system;
          inputs = null;
          nixpkgs = nixpkgs';
        };
      in
      {
        defaultPackage = pkgs.default;

        packages = pkgs;

        devShell = pkgs.devShell;
      }
    );
}
