{
  pkgs,
  perSystem,
  ...
}:
pkgs.mkShellNoCC {
  env.GOROOT = "${pkgs.go}/share/go";

  packages =
    (with pkgs; [
      go
      goreleaser
      golangci-lint
      delve
      pprof
      graphviz
      cobra-cli
      enumer
      perSystem.gomod2nix.default
    ])
    ++ # include formatters for development and testing
    (import ../packages/treefmt/formatters.nix pkgs);
}
