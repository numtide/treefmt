pkgs:
with pkgs; [
  alejandra
  elmPackages.elm-format
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
  # util for unit testing
  (pkgs.writeShellApplication {
    name = "test-fmt";
    text = ''
      VALUE="$1"
      shift

      # append value to each file
      for FILE in "$@"; do
          echo "$VALUE" >> "$FILE"
      done
    '';
  })
]
