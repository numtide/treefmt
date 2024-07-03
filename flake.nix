{
  description = "Treefmt: once CLI to format your repo";

  nixConfig = {
    extra-substituters = ["https://numtide.cachix.org"];
    extra-trusted-public-keys = ["numtide.cachix.org-1:2ps1kLBUWjxIneOy1Ik6cQjb41X0iXVXeHigGmycPPE="];
  };

  inputs = {
    blueprint = {
      url = "github:numtide/blueprint";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    devshell = {
      url = "github:numtide/devshell";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    gomod2nix = {
      url = "github:nix-community/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.flake-utils.follows = "devshell/flake-utils";
    };
    flake-compat.url = "github:nix-community/flake-compat";
    nix-filter.url = "github:numtide/nix-filter";
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    treefmt-nix = {
      url = "github:numtide/treefmt-nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = inputs: let
    inherit (inputs.nixpkgs) lib;
  in
    inputs.blueprint {
      inherit inputs;
      prefix = "nix/";
      nixpkgs.config = {
        allowUnfreePredicate = pkg: builtins.elem (lib.getName pkg) ["terraform"];
      };
    };
}
