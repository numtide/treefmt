{ system ? builtins.currentSystem }:
(import ./. { inherit system; }).devShell
