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
      npmDepsHash = "sha256-acT9uaUhvxyM/S3hv1M9h5h2H5EpzrNbaxCYmzYn100=";

      npmBuildScript = "docs:build";

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
          command = "cd $PRJ_ROOT/docs && npm run docs:dev";
        }
        {
          inherit category;
          name = "docs:build";
          help = "create a production build of docs";
          command = "cd $PRJ_ROOT/docs && npm run docs:build";
        }
        {
          inherit category;
          name = "docs:preview";
          help = "preview a production build of docs";
          command = "cd $PRJ_ROOT/docs && npm run docs:preview";
        }
        {
          inherit category;
          package = pkgs.vhs;
          help = "generate terminal gifs";
        }
        {
          category = "docs";
          help = "regenerate gifs for docs";
          name = "gifs";
          command = ''
            set -xeuo pipefail

            for tape in $PRJ_ROOT/docs/vhs/*; do
                vhs $tape -o "$PRJ_ROOT/docs/public/$(basename $tape .tape).gif"
            done
          '';
        }
      ];
    };
  };
}
