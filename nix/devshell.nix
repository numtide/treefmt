{inputs, ...}: {
  imports = [
    inputs.devshell.flakeModule
  ];

  config.perSystem = {
    pkgs,
    config,
    ...
  }: {
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

      packages = with pkgs;
        [
          # golang
          go
          delve
        ]
        ++
        # include formatters for development and testing
        (import ./formatters.nix pkgs);

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
