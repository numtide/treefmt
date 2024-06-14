{inputs, ...}: {
  imports = [
    inputs.treefmt-nix.flakeModule
  ];
  perSystem = {
    config,
    self',
    ...
  }: {
    treefmt.config = {
      flakeCheck = true;
      flakeFormatter = true;
      projectRootFile = "flake.nix";

      package = self'.packages.default;

      programs = {
        alejandra.enable = true;
        deadnix.enable = true;
        gofumpt.enable = true;
        prettier.enable = true;
        statix.enable = true;
      };

      settings = {
        global.excludes = [
          "LICENSE"
          # let's not mess with the test folder
          "test/*"
          # unsupported extensions
          "*.{gif,png,svg,tape,mts,lock,mod,sum,toml,env,envrc,gitignore}"
        ];

        formatter = {
          deadnix = {
            priority = 1;
          };

          statix = {
            priority = 2;
          };

          alejandra = {
            priority = 3;
          };

          prettier = {
            options = ["--tab-width" "4"];
            includes = ["*.{css,html,js,json,jsx,md,mdx,scss,ts,yaml}"];
          };
        };
      };
    };

    devshells.default = {
      commands = [
        {
          category = "formatting";
          name = "fmt";
          help = "format the repo";
          command = "nix fmt";
        }
      ];
    };
  };
}
