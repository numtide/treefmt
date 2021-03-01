{ system ? builtins.currentSystem }:
(import ./flake-compat.nix { inherit system; }).shellNix
