{
  pkgs,
  perSystem,
  ...
}:
perSystem.self.treefmt.overrideAttrs (old: {
  GOROOT = "${old.go}/share/go";

  shellHook = ''
    # this is only needed for hermetic builds
    unset GO_NO_VENDOR_CHECKS GOSUMDB GOPROXY GOFLAGS
  '';

  nativeBuildInputs =
    old.nativeBuildInputs
    ++ [
      pkgs.goreleaser
      pkgs.golangci-lint
      pkgs.delve
      pkgs.pprof
      pkgs.graphviz
    ]
    ++
    # include formatters for development and testing
    (import ../packages/treefmt/formatters.nix pkgs);
})
