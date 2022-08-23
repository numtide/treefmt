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

        # In Nix 2.8 you can run `nix fmt` to format this whole repo. Note that you need to have loaded the
        # `nix develop` shell before so the various formatters are available in the PATH.
        # It also assumes that the project root has a flake.nix (override this by setting `projectRootFile`).
        formatter = pkgs.treefmt.withConfig {
          settings = nixpkgs.lib.importTOML ./treefmt.toml;
          wrapper.projectRootFile = "flake.nix";
        };

        devShell = pkgs.devShell;
      }
    );
}
