{
  self,
  inputs,
  ...
}: {
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
        # there's no good way of tying in the version to a git tag or branch
        # so for simplicity's sake we set the version as the commit revision hash
        version = self.shortRev or self.dirtyShortRev;

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
              ".github/"
              "README.md"
              "default.nix"
              "shell.nix"
              ".envrc"
            ];
          };

        modules = ../gomod2nix.toml;

        CGO_ENABLED = 1;

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
