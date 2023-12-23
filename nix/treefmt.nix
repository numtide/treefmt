{inputs, ...}: {
  imports = [
    inputs.treefmt-nix.flakeModule
  ];
  perSystem = {config, ...}: {
    treefmt.config = {
      inherit (config.flake-root) projectRootFile;
      flakeCheck = true;
      flakeFormatter = true;
      programs = {
        alejandra.enable = true;
        deadnix.enable = true;
        gofumpt.enable = true;
        prettier.enable = true;
        statix.enable = true;
      };

      settings.formatter.prettier.options = ["--tab-width" "4"];
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
