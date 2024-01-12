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

        nativeBuildInputs =
          # we need some formatters available for the tests
          (import ./formatters.nix pkgs);

        preCheck = ''
          XDG_CACHE_HOME=$(mktemp -d)
          export XDG_CACHE_HOME
        '';

        meta = with lib; {
          description = "treefmt: one CLI to format your repo";
          homepage = "https://git.numtide.com/numtide/treefmt";
          license = licenses.mit;
          mainProgram = "treefmt";
        };
      };

      default = treefmt;
    };

    overlayAttrs = self'.packages;
  };
}
