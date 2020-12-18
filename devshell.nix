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
  commands = [];

  bash = {
    extra = ''
      export LD_INCLUDE_PATH="$DEVSHELL_DIR/include"
      export LD_LIB_PATH="$DEVSHELL_DIR/lib"
    '';
  };

  env = {};

  packages = [
    # build tools
    ## Rust
    allfmt.rust

    ### Others
    # binutils
    # pkgconfig
    # openssl
    # openssl.dev
    # gcc
    # glibc
    # gmp.dev
    nixpkgs-fmt
  ];
}
