{pkgs, ...}:
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
    ])
    ++ # include formatters for development and testing
    (import ../packages/treefmt/formatters.nix pkgs);
}
