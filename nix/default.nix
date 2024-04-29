{inputs, ...}: {
  imports = [
    inputs.flake-root.flakeModule
    ./checks.nix
    ./devshell.nix
    ./docs.nix
    ./nixpkgs.nix
    ./packages.nix
    ./treefmt.nix
  ];
}
