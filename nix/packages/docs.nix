{ pkgs, perSystem, ... }:
pkgs.stdenvNoCC.mkDerivation {
  name = "docs";

  unpackPhase = ''
    cp ${../../mkdocs.yml} mkdocs.yaml
    cp -r ${../../docs} docs
  '';

  nativeBuildInputs = with pkgs;
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
          ++ (import ./treefmt/formatters.nix pkgs);
        text = ''vhs "$@"'';
      })
    ];

  buildPhase = ''
    mkdocs build
  '';

  installPhase = ''
    mv site $out
  '';
}
