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
        version = "2.0.0+dev";

        # ensure we are using the same version of go to build with
        inherit (pkgs) go;

        src = let
          filter = inputs.nix-filter.lib;
        in
          filter {
            root = ../.;
            exclude = [
              "nix/"
              "docs/"
              "README.md"
              "default.nix"
              "shell.nix"
              ".envrc"
            ];
          };

        modules = ../gomod2nix.toml;

        ldflags = [
          "-s"
          "-w"
          "-X git.numtide.com/numtide/treefmt/build.Name=${pname}"
          "-X git.numtide.com/numtide/treefmt/build.Version=v${version}"
        ];

        nativeBuildInputs =
          # we need some formatters available for the tests
          import ./formatters.nix pkgs;

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
