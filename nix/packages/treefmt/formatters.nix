pkgs:
with pkgs; [
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
]
