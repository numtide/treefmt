final: prev:
{
  prjfmt = rec {
    bin = prev.rustPlatform.buildRustPackage rec {
      pname = "prjfmt";
      version = "0.0.1";
      src = ./.;
      cargoSha256 = "sha256-98M9OPWil9bKknam8ys4dNP6/iZObW0RrAC7PxiHxYI=";
      doCheck = false;
      nativeBuildInputs = [ ];
      buildInputs = [ ];
    };
  };
}
