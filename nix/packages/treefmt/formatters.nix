pkgs:
with pkgs; [
  bash # used in tree-root-cmd tests

  # rest are formatters
  alejandra
  gotools
  haskellPackages.cabal-fmt
  haskellPackages.ormolu
  mdsh
  nixpkgs-fmt
  nodePackages.prettier
  python3.pkgs.black
  rufo
  rustfmt
  shellcheck
  shfmt
  statix
  deadnix
  opentofu
  dos2unix
  yamlfmt
  # utils for unit testing
  (pkgs.writeShellApplication {
    name = "test-fmt-append";
    text = ''
      VALUE="$1"
      shift

      # append value to each file
      for FILE in "$@"; do
          echo "$VALUE" >> "$FILE"
      done
    '';
  })
  (pkgs.writeShellApplication {
    name = "test-fmt-modtime";
    text = ''
      VALUE="$1"
      shift

      # append value to each file
      for FILE in "$@"; do
          touch -t "$VALUE" "$FILE"
      done
    '';
  })
  (pkgs.writeShellApplication {
    name = "test-fmt-one-path";
    text = ''
      if [ $# -gt 1 ]; then
        echo "Error: Too many arguments"
        exit 1
      fi

      test-fmt-four-paths
    '';
  })
  (pkgs.writeShellApplication {
    name = "test-fmt-two-paths";
    text = ''
      if [ $# -gt 2 ]; then
        echo "Error: Too many arguments"
        exit 1
      fi

      test-fmt-four-paths
    '';
  })
  (pkgs.writeShellApplication {
    name = "test-fmt-four-paths";
    text = ''
      if [ $# -gt 4 ]; then
        echo "Error: Too many arguments"
        exit 1
      fi

      test-fmt-append "foo" "$@"
    '';
  })
]
