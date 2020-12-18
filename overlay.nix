final: prev:
{
  allfmt = rec {
    rust = prev.callPackage ./rust { };
  };
}
