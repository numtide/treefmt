{ system ? builtins.currentSystem }:
let outputs = (import ./flake-compat.nix {
  inherit system;
}).defaultNix;
in
{
  treefmt = outputs.defaultPackage.${system};
  docs = outputs.docs.${system};
}
