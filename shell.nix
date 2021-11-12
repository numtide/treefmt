{ system ? builtins.currentSystem }:
(import ./. { inherit system; }).treefmt
