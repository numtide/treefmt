{lib, ...}: {
  perSystem = {self', ...}: {
    checks = with lib; mapAttrs' (n: nameValuePair "package-${n}") self'.packages;
  };
}
