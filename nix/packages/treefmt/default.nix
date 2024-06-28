{
  pname,
  pkgs,
  flake,
  inputs,
  perSystem,
  ...
}: let
  inherit (pkgs) lib;
in
  perSystem.gomod2nix.buildGoApplication rec {
    inherit pname;
    # there's no good way of tying in the version to a git tag or branch
    # so for simplicity's sake we set the version as the commit revision hash
    # we remove the `-dirty` suffix to avoid a lot of unnecessary rebuilds in local dev
    version = lib.removeSuffix "-dirty" (flake.shortRev or flake.dirtyShortRev);

    # ensure we are using the same version of go to build with
    inherit (pkgs) go;

    src = let
      filter = inputs.nix-filter.lib;
    in
      filter {
        root = ../../../.;
        exclude = [
          "nix/"
          "docs/"
          ".github/"
          "README.md"
          "default.nix"
          "shell.nix"
          ".env"
          ".envrc"
        ];
      };

    modules = ./gomod2nix.toml;

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
  }
