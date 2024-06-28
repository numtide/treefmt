{
  pkgs,
  perSystem,
  ...
}: let
  inherit (pkgs) lib;
  inherit (pkgs.stdenv) isLinux isDarwin;
in
  perSystem.devshell.mkShell {
    env = [
      {
        name = "GOROOT";
        value = pkgs.go + "/share/go";
      }
      {
        name = "LD_LIBRARY_PATH";
        value = "$DEVSHELL_DIR/lib";
      }
    ];

    packages = lib.mkMerge [
      (with pkgs; [
        # golang
        go
        goreleaser
        golangci-lint
        delve
        pprof
        graphviz

        # docs
        nodejs
      ])
      # platform dependent CGO dependencies
      (lib.mkIf isLinux [pkgs.gcc])
      (lib.mkIf isDarwin [pkgs.darwin.cctools])
      # include formatters for development and testing
      (import ./packages/treefmt/formatters.nix pkgs)
    ];

    commands = [
      {package = perSystem.gomod2nix.default;}
      {
        name = "docs:dev";
        help = "serve docs for local development";
        command = "cd $PRJ_ROOT/docs && npm ci && npm run dev";
      }
      {
        name = "docs:build";
        help = "create a production build of docs";
        command = "cd $PRJ_ROOT/docs && npm ci && npm run build";
      }
      {
        name = "docs:preview";
        help = "preview a production build of docs";
        command = "cd $PRJ_ROOT/docs && npm ci && npm run preview";
      }
      {
        help = "generate terminal gifs";
        package = pkgs.writeShellApplication {
          name = "vhs";
          runtimeInputs =
            [
              perSystem.self.treefmt
              pkgs.rsync
              pkgs.vhs
            ]
            ++ (import ./packages/treefmt/formatters.nix pkgs);
          text = ''vhs "$@"'';
        };
      }
    ];
  }
