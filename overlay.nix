final: prev:
{
  prjfmt = rec {
    bin = prev.rustPlatform.buildRustPackage rec {
      pname = "prjfmt";
      version = "0.0.1";
      src = ./.;
      cargoSha256 = "0q7f596yvzbsg3g8x47cazfrfpa4j3310z14mz1w6i2xjvmadgx6";
      doCheck = false;
      nativeBuildInputs = [ ];
      buildInputs = [ ];
    };
  };
}
