{
  description = "todomvc-nix";
  # To update all inputs:
  # $ nix flake update --recreate-lock-file
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/master";
  inputs.flake-utils.url = "github:numtide/flake-utils";
  inputs.flake-utils.inputs.nixpkgs.follows = "nixpkgs";
  inputs.devshell.url = "github:numtide/devshell/master";
  inputs.devshell.inputs.nixpkgs.follows = "nixpkgs";

  # Only for example, use the .url for simplicity
  inputs.mozilla-overlay.url = "github:mozilla/nixpkgs-mozilla";
  inputs.mozilla-overlay.flake = false;

  outputs = { self, nixpkgs, mozilla-overlay, flake-utils, devshell }:
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
              (import mozilla-overlay)
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
