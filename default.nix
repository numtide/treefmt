{ system ? builtins.currentSystem }:
(import ./flake-compat.nix {
  inherit system;
}).defaultNix.defaultPackage.${system}
