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
  bash = {
    extra = ''
      export LD_INCLUDE_PATH="$DEVSHELL_DIR/include"
      export LD_LIB_PATH="$DEVSHELL_DIR/lib"
    '';
    interactive = '''';
  };
  packages = [
    # Build tools
    (rust-bin.stable.latest.rust.override {
      extensions = [ "rust-src" ];
    })
    clang

    # Code formatters
    haskellPackages.ormolu
    haskellPackages.cabal-install
    haskellPackages.ghc
    nixpkgs-fmt
    go
    gopls
    gopkgs
    python3.pkgs.black
    nodePackages.prettier
  ];
}
