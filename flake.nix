{
  description = "treefmt";
  # To update all inputs:
  # $ nix flake update --recreate-lock-file
  inputs.devshell.url = "github:numtide/devshell";
  inputs.flake-utils.url = "github:numtide/flake-utils";
  inputs.naersk.inputs.nixpkgs.follows = "nixpkgs";
  inputs.naersk.url = "github:nmattia/naersk";

  outputs = { self, nixpkgs, naersk, flake-utils, devshell }@inputs:
    flake-utils.lib.eachSystem [ "x86_64-linux" "x86_64-darwin" ] (system:
      let pkgs = import ./. { inherit system inputs; }; in
      {
        defaultPackage = pkgs.treefmt;
        packages = pkgs;
        devShell = pkgs.devShell;
      }
    );
}
