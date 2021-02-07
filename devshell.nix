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

  # Needed by rust-analyzer
  env.RUST_SRC_PATH = rustPackages.rustPlatform.rustLibSrc;

  packages = [
    # Build tools
    rustPackages.cargo
    rustPackages.clippy
    rustPackages.rustc
    clang
    rust-analyzer

    # Code formatters
    elmPackages.elm-format
    go
    gopkgs
    gopls
    haskellPackages.cabal-install
    haskellPackages.ghc
    haskellPackages.ormolu
    nixpkgs-fmt
    nodePackages.prettier
    python3.pkgs.black
    rustPackages.rustfmt
    shfmt
  ];
}
