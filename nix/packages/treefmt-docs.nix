{
  pname,
  pkgs,
  flake,
  perSystem,
  ...
}:
pkgs.buildNpmPackage {
  inherit pname;
  inherit (perSystem.self.treefmt) version;

  src = "${flake}/docs";
  npmDepsHash = "sha256-+UEtUKefaFZm1a0rAHBa7k7uysYKPscQa1PMdR1MJ2o=";

  # we have to use a custom build phase because vitepress is doing something funky with the ttty
  buildPhase = ''
    cat | npm run build 2>&1 | cat
  '';

  installPhase = ''
    runHook preInstall
    cp -rv .vitepress/dist/ $out
    runHook postInstall
  '';
}
