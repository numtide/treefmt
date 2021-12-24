{ mdbook, stdenv }:

stdenv.mkDerivation {
  name = "treefmt-docs";
  buildInputs = [ mdbook ];
  src = ./.;
  buildPhase = "mdbook build";
  installPhase = ''
    mv book $out
  '';
}
