final: prev:
{
  prjfmt = rec {
    bin = prev.rustPlatform.buildRustPackage rec {
      pname = "prjfmt";
      version = "0.0.1";
      src = ./.;
      cargoSha256 = "1n9wzbv6nc5sh2l0j1xfb2z65ql2krqw2ba9s7krlfhsbdkndwrr";
      doCheck = false;
      nativeBuildInputs = [ ];
      buildInputs = [ ];
    };
  };
}
