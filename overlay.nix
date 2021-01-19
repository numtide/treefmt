final: prev:
{
  prjfmt = rec {
    rust = prev.callPackage ./rust { };
  };
}
