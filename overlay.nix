final: prev:
{
  prjfmt = rec {
    bin = prev.rustPlatform.buildRustPackage rec {
      pname = "prjfmt";
      version = "0.0.1";
      src = ./.;
      cargoSha256 = "sha256:0bsxwl5bhjg8d4mdyk2mx7g5744b790nhdlyg393vwjmnbnyy1k2";
      doCheck = false;
      nativeBuildInputs = [ ];
      buildInputs = [ ];
    };
  };
}
