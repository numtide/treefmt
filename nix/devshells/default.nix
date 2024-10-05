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
    ++ (with pkgs; [
      goreleaser
      golangci-lint
      delve
      pprof
      graphviz
      cobra-cli
      enumer
      perSystem.gomod2nix.default
    ])
    ++
    # include formatters for development and testing
    (import ../packages/treefmt/formatters.nix pkgs);
})
