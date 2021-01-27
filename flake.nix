{
  description = "todomvc-nix";
  # To update all inputs:
  # $ nix flake update --recreate-lock-file
  inputs.flake-utils.url = "github:numtide/flake-utils";
  inputs.devshell.url = "github:numtide/devshell";
  inputs.rust-overlay.url = "github:oxalica/rust-overlay";

  outputs = { self, nixpkgs, rust-overlay, flake-utils, devshell }:
    {
      overlay = import ./overlay.nix;
    }
    //
    (
      flake-utils.lib.eachSystem [ "x86_64-linux" ] (system:
        let
          pkgs = import nixpkgs {
            inherit system;
            # Makes the config pure as well. See <nixpkgs>/top-level/impure.nix:
            config = {
              allowBroken = true;
              permittedInsecurePackages = [
                "openssl-1.0.2u"
              ];
            };
            overlays = [
              rust-overlay.overlay
              devshell.overlay
              self.overlay
            ];
          };
        in
        {
          legacyPackages = pkgs.prjfmt;

          packages = flake-utils.lib.flattenTree pkgs.prjfmt;

          devShell = import ./devshell.nix { inherit pkgs; };

          checks = { };
        }
      )
    );
}
