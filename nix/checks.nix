{
  lib,
  inputs,
  self,
  ...
}: {
  # generate github actions matrix using the flake's checks
  flake.githubActions = inputs.nix-github-actions.lib.mkGithubMatrix {
    checks = lib.getAttrs ["x86_64-linux" "x86_64-darwin"] self.checks;
  };

  perSystem = {self', ...}: {
    # mixin every package
    checks = with lib; mapAttrs' (n: nameValuePair "package-${n}") self'.packages;
  };
}
