{ stdenv, mkdocs, python310Packages }:

stdenv.mkDerivation {
  name = "treefmt-docs";

  src = builtins.path {
    name = "treefmt-docs";
    path = ./.;
  };

  # Re-create the folder structure since mkdocs insists on having the
  # mkdocs.yml at the root of the repo.
  unpackPhase = ''
    cp -r --no-preserve=mode $src docs
    cp ${../mkdocs.yml} mkdocs.yml
  '';

  nativeBuildInputs = [
    mkdocs
    python310Packages.mkdocs-material
  ];

  buildPhase = ''
    mkdocs build
  '';

  installPhase = ''
    mv site $out
  '';
}
