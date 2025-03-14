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
  pkgs.callPackage ./package.nix {
    inherit pname;
    # There's no good way of tying in the version to a git tag or branch,
    # so for simplicity's sake we set the version as the commit revision hash.
    # We remove the `-dirty` suffix to avoid a lot of unnecessary rebuilds in local dev.
    version = lib.removeSuffix "-dirty" (flake.shortRev or flake.dirtyShortRev);

    buildGoModule = pkgs.buildGo124Module;

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

    passthru = let
      inherit (perSystem.self) treefmt;
    in {
      no-vendor-hash = treefmt.overrideAttrs {
        vendorHash = "";
      };

      update-vendor-hash = pkgs.writeShellApplication {
        name = "update-vendor-hash";
        runtimeInputs = with pkgs; [
          nix
          coreutils
          gnused
          gawk
        ];
        text = ''
          ROOT_DIR=''${1:-.}

          FAILED_BUILD=$(nix build .#treefmt.no-vendor-hash 2>&1 || true)
          echo "$FAILED_BUILD"
          CHECKSUM=$(echo "$FAILED_BUILD" | awk '/got:.*sha256/ { print $2 }')

          sed -i -e "s|vendorHash = \".*\"|vendorHash = \"$CHECKSUM\"|" "$ROOT_DIR/nix/packages/treefmt/package.nix"
        '';
      };

      tests = {
        coverage = lib.optionalAttrs pkgs.stdenv.isx86_64 (treefmt.overrideAttrs (old: {
          nativeBuildInputs =
            old.nativeBuildInputs
            ++ [pkgs.git pkgs.gcc]
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

          env.CGO_ENABLED = 1;

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
    };
  }
