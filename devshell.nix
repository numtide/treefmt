{ pkgs }:

with pkgs;

# Configure your development environment.
#
# Documentation: https://github.com/numtide/devshell
devshell.mkShell {
  name = "prjfmt";
  motd = ''
    Welcome to the prjfmt development environment.
  '';
  commands = [ ];
  packages = [
    # Build tools
    rust-bin.stable.latest.rust
    clang

    # Code formatters
    haskellPackages.ormolu
    haskellPackages.cabal-install
    haskellPackages.ghc
    nixpkgs-fmt
    go
    gopls
    gopkgs
    gocode
  ];
}
