{
  pkgs,
  perSystem,
  ...
}:
pkgs.mkShellNoCC {
  packages = let
    docs = command:
      pkgs.writeShellApplication {
        name = "docs:${command}";
        runtimeInputs = [pkgs.nodejs];
        text = ''cd "''${DIRENV_DIR:1}/docs" && npm ci && npm run ${command}'';
      };
  in [
    (docs "dev")
    (docs "build")
    (docs "preview")
    (pkgs.writeShellApplication {
      name = "vhs";
      runtimeInputs =
        [
          perSystem.self.treefmt
          pkgs.rsync
          pkgs.vhs
        ]
        ++ (import ../packages/treefmt/formatters.nix pkgs);
      text = ''vhs "$@"'';
    })
  ];
}
