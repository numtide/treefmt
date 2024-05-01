{
  description = "Treefmt";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    flake-parts.url = "github:hercules-ci/flake-parts";
    flake-root.url = "github:srid/flake-root";
    treefmt-nix = {
      # todo switch back to numtide/treefmt-nix once merged
      url = "github:brianmcgee/treefmt-nix/feat/pipelines";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    devshell = {
      url = "github:numtide/devshell";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    gomod2nix = {
      url = "github:nix-community/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    nix-filter.url = "github:numtide/nix-filter";
    flake-compat.url = "github:nix-community/flake-compat";
  };

  outputs = inputs @ {flake-parts, ...}:
    flake-parts.lib.mkFlake
    {
      inherit inputs;
    }
    {
      imports = [
        ./nix
      ];
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
    };
}
