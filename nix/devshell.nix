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

      packages = with pkgs; [
        # golang
        go
        go-tools
        delve
        golangci-lint

        # formatters for testing

        elmPackages.elm-format
        haskellPackages.cabal-fmt
        haskellPackages.ormolu
        mdsh
        nixpkgs-fmt
        nodePackages.prettier
        python3.pkgs.black
        rufo
        rustfmt
        shellcheck
        shfmt
        terraform
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
