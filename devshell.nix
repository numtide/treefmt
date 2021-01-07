{ pkgs }:

with pkgs;

# Configure your development environment.
#
# Documentation: https://github.com/numtide/devshell
mkDevShell {
  name = "allfmt";
  motd = ''
    Welcome to the allfmt development environment.
  '';
  commands = [ ];

  bash = {
    extra = ''
      export LD_INCLUDE_PATH="$DEVSHELL_DIR/include"
      export LD_LIBRARY_PATH="$DEVSHELL_DIR/lib"
      export PKG_CONFIG_PATH="$DEVSHELL_DIR/lib/pkgconfig"
    '';
  };

  env = { };

  packages = [
    # Build tools
    allfmt.rust

    # Code formatters
    haskellPackages.ormolu
    haskellPackages.cabal-install
    haskellPackages.ghc
    nixpkgs-fmt
  ];
}
