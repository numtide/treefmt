{
  pkgs,
  pname,
  perSystem,
  ...
}:
perSystem.devshell.mkShell {
  name = pname;

  packages = [
    pkgs.git
    perSystem.gomod2nix.default
  ];

  commands = [
    {
      name = "gomod2nix:update";
      help = "update gomod2nix.toml";
      command = "gomod2nix --dir $PRJ_ROOT --outdir $PRJ_ROOT/nix/packages/treefmt";
    }
  ];
}
