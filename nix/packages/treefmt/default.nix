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
  pkgs.buildGoModule rec {
    inherit pname;
    # there's no good way of tying in the version to a git tag or branch
    # so for simplicity's sake we set the version as the commit revision hash
    # we remove the `-dirty` suffix to avoid a lot of unnecessary rebuilds in local dev
    version = lib.removeSuffix "-dirty" (flake.shortRev or flake.dirtyShortRev);

    # ensure we are using the same version of go to build with
    go = pkgs.go_1_23;

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

    vendorHash = "sha256-LQFb0X6clsVadMAWz+Pl7ONa033+faJxbvfkY/3MEBo=";

    CGO_ENABLED = 0;

    ldflags = [
      "-s"
      "-w"
      "-X github.com/numtide/treefmt/v2/build.Name=${pname}"
      "-X github.com/numtide/treefmt/v2/build.Version=v${version}"
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

    passthru.tests = let
      inherit (perSystem.self) treefmt;
    in {
      coverage = lib.optionalAttrs pkgs.stdenv.isx86_64 (treefmt.overrideAttrs (old: {
        nativeBuildInputs = old.nativeBuildInputs ++ [pkgs.gcc];
        CGO_ENABLED = 1;
        buildPhase = ''
          HOME=$TMPDIR
          go test -race -covermode=atomic -coverprofile=coverage.out -v ./...
        '';
        installPhase = ''
          mv coverage.out $out
        '';
      }));

      golangci-lint = treefmt.overrideAttrs (old: {
        nativeBuildInputs = old.nativeBuildInputs ++ [pkgs.golangci-lint];
        buildPhase = ''
          HOME=$TMPDIR
          golangci-lint run
        '';
        installPhase = ''
          touch $out
        '';
      });
    };

    meta = with lib; {
      description = "treefmt: the formatter multiplexer";
      homepage = "https://github.com/numtide/treefmt";
      license = licenses.mit;
      mainProgram = "treefmt";
    };
  }
