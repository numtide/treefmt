{ pkgs }:

with pkgs;

# Configure your development environment.
#
# Documentation: https://github.com/numtide/devshell
mkDevShell {
  name = "prjfmt";
  motd = ''
    Welcome to the prjfmt development environment.
  '';
  commands = [ ];

  bash = {
    extra = ''
      export LD_INCLUDE_PATH="$DEVSHELL_DIR/include"
      export LD_LIBRARY_PATH="$DEVSHELL_DIR/lib"
      export PKG_CONFIG_PATH="$DEVSHELL_DIR/lib/pkgconfig"
      export GO111MODULE=on
      unset GOPATH GOROOT
    '';
  };

  env = { };

  packages = [
    # Build tools
    prjfmt.rust

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
