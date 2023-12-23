{inputs, ...}: {
  perSystem = {system, ...}: {
    # customise nixpkgs instance
    _module.args.pkgs = import inputs.nixpkgs {
      inherit system;
      overlays = [
        inputs.gomod2nix.overlays.default
      ];
      config = {
        # for terraform
        # todo make this more specific
        allowUnfree = true;
      };
    };
  };
}
