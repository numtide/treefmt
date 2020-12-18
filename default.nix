{ system ? builtins.currentSystem }:
let
  flake-compat = builtins.fetchurl {
    url = "https://raw.githubusercontent.com/edolstra/flake-compat/99f1c2157fba4bfe6211a321fd0ee43199025dbf/default.nix";
    sha256 = "1vas5z58901gavy5d53n1ima482yvly405jp9l8g07nr4abmzsyb";
  };
in
import flake-compat {
  src = ./.;
  inherit system;
}
