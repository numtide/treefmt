_: {
  perSystem = {pkgs, ...}: {
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
                vhs $tape -o "$PRJ_ROOT/docs/assets/$(basename $tape .tape).gif"
            done
          '';
        }
      ];
    };
  };
}
