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
  pkgs.buildGo124Module rec {
    inherit pname;
    # there's no good way of tying in the version to a git tag or branch
    # so for simplicity's sake we set the version as the commit revision hash
    # we remove the `-dirty` suffix to avoid a lot of unnecessary rebuilds in local dev
    version = lib.removeSuffix "-dirty" (flake.shortRev or flake.dirtyShortRev);

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

    vendorHash = "sha256-Xc3T3gh5Wpo3kgrQdLDXcgV0VM4VQTTwg+bbiTTzbzs=";

    env.CGO_ENABLED = 0;

    ldflags = [
      "-s"
      "-w"
      "-X github.com/numtide/treefmt/v2/build.Name=${pname}"
      "-X github.com/numtide/treefmt/v2/build.Version=v${version}"
    ];

    nativeBuildInputs =
      (with pkgs; [
        git
        installShellFiles
      ])
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

    postInstall = ''
      export HOME=$PWD

      installShellCompletion --cmd treefmt \
          --bash <($out/bin/treefmt --completion bash) \
          --fish <($out/bin/treefmt --completion fish) \
          --zsh <($out/bin/treefmt --completion zsh)
    '';

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

          # only replace the first entry in the file so we don't break no-vendor-hash
          sed -i -e "0,/vendorHash = \".*\"/s|vendorHash = \".*\"|vendorHash = \"$CHECKSUM\"|" "$ROOT_DIR/nix/packages/treefmt/default.nix"
        '';
      };

      tests = {
        coverage = lib.optionalAttrs pkgs.stdenv.isx86_64 (treefmt.overrideAttrs (old: {
          nativeBuildInputs = old.nativeBuildInputs ++ [pkgs.gcc];
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

    meta = with lib; {
      description = "treefmt: the formatter multiplexer";
      homepage = "https://github.com/numtide/treefmt";
      license = licenses.mit;
      mainProgram = "treefmt";
    };
  }
