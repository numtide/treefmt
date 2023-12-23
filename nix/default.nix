{inputs, ...}: {
  imports = [
    inputs.flake-root.flakeModule
    ./checks.nix
    ./devshell.nix
    ./nixpkgs.nix
    ./packages.nix
    ./treefmt.nix
  ];
}
