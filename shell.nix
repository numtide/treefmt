{ system ? builtins.currentSystem }:
(import ./. { inherit system; }).shellNix
