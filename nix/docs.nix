_: {
  perSystem = {
    pkgs,
    self',
    ...
  }: {
    packages.docs = pkgs.buildNpmPackage {
      pname = "treefmt-docs";
      inherit (self'.packages.default) version;

      src = ../docs;
      npmDepsHash = "sha256-vHxJHuhvQJ0P4wS1Hd2BIfPMSptnLhuHGLXCO+P5iFs=";

      # we have to use a custom build phase because vitepress is doing something funky with the ttty
      buildPhase = ''
        cat | npm run build 2>&1 | cat
      '';

      installPhase = ''
        runHook preInstall
        cp -rv .vitepress/dist/ $out
        runHook postInstall
      '';
    };

    devshells.default = {
      packages = [
        pkgs.nodejs
      ];

      commands = let
        category = "docs";
      in [
        {
          inherit category;
          name = "docs:dev";
          help = "serve docs for local development";
          command = "cd $PRJ_ROOT/docs && npm run dev";
        }
        {
          inherit category;
          name = "docs:build";
          help = "create a production build of docs";
          command = "cd $PRJ_ROOT/docs && npm run build";
        }
        {
          inherit category;
          name = "docs:preview";
          help = "preview a production build of docs";
          command = "cd $PRJ_ROOT/docs && npm run preview";
        }
        {
          inherit category;
          help = "generate terminal gifs";
          package = pkgs.writeShellApplication {
            name = "vhs";
            runtimeInputs =
              [
                self'.packages.treefmt
                pkgs.rsync
                pkgs.vhs
              ]
              ++ (import ./formatters.nix pkgs);
            text = ''vhs "$@"'';
          };
        }
      ];
    };
  };
}
