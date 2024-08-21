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
      text = "gomod2nix --outdir nix/packages/treefmt";
    })
  ];
}
