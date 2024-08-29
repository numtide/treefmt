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
  npmDepsHash = "sha256-0WV+aLYmu7/CHpKbffmKfxd8Vd5XAXra+Ht0RZlzOpY=";

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
