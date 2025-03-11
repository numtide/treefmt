{pkgs, ...}:
pkgs.mkShellNoCC {
  env = {
    GOROOT = "${pkgs.go_1_24}/share/go";
    CGO_ENABLED = "0";
  };

  packages =
    (with pkgs; [
      go_1_24
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
