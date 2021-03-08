{ system ? builtins.currentSystem }:
let
  flake-compat = import ./flake-compat.nix {
    inherit system;
  };
in
  flake-compat.defaultNix.legacyPackages.${system} or
    (throw "The system '${system}' is not supported. Please open an issue!")
