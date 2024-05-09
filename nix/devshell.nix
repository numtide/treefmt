{inputs, ...}: {
  imports = [
    inputs.devshell.flakeModule
  ];

  config.perSystem = {
    lib,
    pkgs,
    config,
    ...
  }: let
    inherit (pkgs.stdenv) isLinux isDarwin;
  in {
    config.devshells.default = {
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
          delve
          pprof
          graphviz
        ])
        # platform dependent CGO dependencies
        (lib.mkIf isLinux [
          pkgs.gcc
        ])
        (lib.mkIf isDarwin [
          pkgs.darwin.cctools
        ])
        # include formatters for development and testing
        (import ./formatters.nix pkgs)
      ];

      commands = [
        {
          category = "development";
          package = pkgs.gomod2nix;
        }
        {
          category = "development";
          package = pkgs.enumer;
        }
      ];
    };
  };
}
