{inputs, ...}: {
  imports = [
    inputs.flake-parts.flakeModules.easyOverlay
  ];

  perSystem = {
    self',
    inputs',
    lib,
    pkgs,
    ...
  }: {
    packages = rec {
      treefmt = inputs'.gomod2nix.legacyPackages.buildGoApplication rec {
        pname = "treefmt";
        version = "0.0.1+dev";

        # ensure we are using the same version of go to build with
        inherit (pkgs) go;

        src = ../.;
        modules = ../gomod2nix.toml;

        ldflags = [
          "-X 'build.Name=${pname}'"
          "-X 'build.Version=${version}'"
        ];

        meta = with lib; {
          description = "treefmt: one CLI to format your repo";
          homepage = "https://github.com/numtide/treefmt";
          license = licenses.mit;
          mainProgram = "treefmt";
        };
      };

      default = treefmt;
    };

    overlayAttrs = self'.packages;
  };
}
