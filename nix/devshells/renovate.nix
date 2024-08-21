{
  pkgs,
  pname,
  perSystem,
  ...
}:
pkgs.mkShellNoCC {
  inherit pname;
  packages = [
    (pkgs.writeShellApplication {
      name = "gomod2nix:update";
      runtimeInputs = [pkgs.git perSystem.gomod2nix.default];
      text = ''
        PRJ_ROOT=''${DIRENV_DIR:1}
        gomod2nix --dir "$PRJ_ROOT" --outdir "$PRJ_ROOT/nix/packages/treefmt"
      '';
    })
  ];
}
