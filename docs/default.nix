{ mdbook, stdenv }:

stdenv.mkDerivation {
  name = "treefmt-docs";
  buildInputs = [ mdbook ];
  src = ./.;
  buildPhase = "mdbook build";
  installPhase = ''
    mkdir $out
    # Move the landing page as the main HTML. Docbook copies all of the files
    # so it can't be named index.html
    mv landing.html $out/index.html
    mv book $out/docs
  '';
}
