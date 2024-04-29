_: {
  perSystem = {pkgs, self', ...}: {
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
      commands = let
        category = "docs";
      in [
        {
          inherit category;
          package = pkgs.nodejs;
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
