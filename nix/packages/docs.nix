{
  pkgs,
  perSystem,
  ...
}:
pkgs.stdenvNoCC.mkDerivation {
  name = "docs";

  src = ../../.;

  nativeBuildInputs =
    [
      perSystem.self.treefmt
    ]
    ++ (with pkgs.python3Packages; [
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
          ++ (import ./treefmt/formatters.nix pkgs);
        text = ''vhs "$@"'';
      })
    ];

  buildPhase = ''
    cd docs
    mkdocs build
  '';

  installPhase = ''
    mv out $out
  '';
}
