{
  pkgs,
  perSystem,
  ...
}:
pkgs.mkShellNoCC {
  packages = with pkgs;
    (with pkgs.python3Packages; [
      mike
      mkdocs
      mkdocs-material
      mkdocs-awesome-pages-plugin
    ])
    ++ [
      (pkgs.writeShellApplication {
        name = "vhs";
        runtimeInputs =
          [
            perSystem.self.treefmt
            pkgs.rsync
            pkgs.vhs
          ]
          ++ (import ../packages/treefmt/formatters.nix pkgs);
        text = ''vhs "$@"'';
      })
    ];
}
