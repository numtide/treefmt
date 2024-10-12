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

    CGO_ENABLED = 0;

    ldflags = [
      "-s"
      "-w"
      "-X github.com/numtide/treefmt/build.Name=${pname}"
      "-X github.com/numtide/treefmt/build.Version=v${version}"
    ];

    nativeBuildInputs =
      [pkgs.git]
      ++
      # we need some formatters available for the tests
      import ./formatters.nix pkgs;

    preCheck = ''
      HOME=$(mktemp -d)
      XDG_CACHE_HOME=$(mktemp -d)

      export HOME XDG_CACHE_HOME

      # setup a git user for committing during tests
      git config --global user.email "<test@treefmt.com>"
      git config --global user.name "Treefmt Test"
    '';

    meta = with lib; {
      description = "treefmt: one CLI to format your repo";
      homepage = "https://github.com/numtide/treefmt";
      license = licenses.mit;
      mainProgram = "treefmt";
    };
  }
