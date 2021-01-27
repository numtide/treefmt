final: prev:
{
  prjfmt = rec {
    bin = prev.rustPlatform.buildRustPackage rec {
      pname = "prjfmt";
      version = "0.0.1";

      src = ./.;
      cargoSha256 = "sha256-99ioLTIQ2PbfEtrCu89aLnrxN3frx7vHD9D40bHWhbc=";
      doCheck = false;
      nativeBuildInputs = [ ];
      buildInputs = [ ];
    };
  };
}
