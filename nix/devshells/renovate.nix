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
        set -eu
        gomod2nix --outdir nix/packages/treefmt
        # shellcheck disable=SC2016
        sed -i '1i # Generated with `nix develop .#renovate -c gomod2nix:update`' nix/packages/treefmt/gomod2nix.toml
      '';
    })
  ];
}
