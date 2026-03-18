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
  just
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
    name = "test-fmt-delayed-append";
    text = ''
      DELAY="$1"
      shift

      # sleep first
      sleep "$DELAY"

      test-fmt-append "$@"
    '';
  })
  (pkgs.writeShellApplication {
    name = "test-fmt-only-one-file-at-a-time";
    text = ''
      if [ $# -ne 1 ]; then
        echo "I only support formatting exactly 1 file at a time"
        exit 1
      fi

      test-fmt-append "suffix" "$1"
    '';
  })
]
