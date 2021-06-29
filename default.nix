{ system ? builtins.currentSystem
, inputs ? import ./flake.lock.nix { }
}:
let
  nixpkgs = import inputs.nixpkgs {
    inherit system;
    # Makes the config pure as well. See <nixpkgs>/top-level/impure.nix:
    config = { };
  };

  devshell = import inputs.devshell {
    inherit system;
    pkgs = nixpkgs;
  };

  naersk = nixpkgs.callPackage inputs.naersk { };
in
{
  # What is used when invoking `nix run github:numtide/treefmt`
  treefmt = naersk.buildPackage {
    src = nixpkgs.lib.cleanSource ./.;
  };

  # A collection of packages for the project
  docs = nixpkgs.callPackage ./docs { };

  # The development environment
  devShell = devshell.fromTOML ./devshell.toml;
}
