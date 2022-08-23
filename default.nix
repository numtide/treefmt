{ system ? builtins.currentSystem
, inputs ? import ./flake.lock.nix { }
, nixpkgs ? import inputs.nixpkgs {
    inherit system;
    # Makes the config pure as well. See <nixpkgs>/top-level/impure.nix:
    config = { };
    overlays = [ ];
  }
, rustPackages ? nixpkgs.rustPackages
}:
let
  lib = nixpkgs.lib;

  cargoToml = with builtins; (fromTOML (readFile ./Cargo.toml));

  # A new kind of option type that calls lib.getExe on derivations
  exeType = lib.mkOptionType {
    name = "exe";
    description = "Path to executable";
    check = (x: lib.isString x || lib.isPath x || lib.isDerivation x);
    merge = loc: defs:
      let res = lib.mergeOneOption loc defs; in
      if lib.isString res || lib.isPath res then
        "${res}"
      else
        lib.getExe res;
  };

  # The schema of the treefmt.toml data structure.
  configSchema = with lib; {
    excludes = mkOption {
      description = "A global list of paths to exclude. Supports glob.";
      type = types.listOf types.str;
      default = [ ];
      example = [ "./node_modules/**" ];
    };

    formatter = mkOption {
      type = types.attrsOf (types.submodule [{
        options = {
          command = mkOption {
            description = "Executable obeying the treefmt formatter spec";
            type = exeType;
          };

          options = mkOption {
            description = "List of arguments to pass to the command";
            type = types.listOf types.str;
            default = [ ];
          };

          includes = mkOption {
            description = "List of files to include for formatting. Supports globbing.";
            type = types.listOf types.str;
          };

          excludes = mkOption {
            description = "List of files to exclude for formatting. Supports globbing. Takes precedence over the includes.";
            type = types.listOf types.str;
            default = [ ];
          };
        };
      }]);
      default = { };
      description = "Set of formatters to use";
    };
  };

  configFormat = nixpkgs.formats.toml { };

  module = { config, ... }: {
    options = {
      settings = configSchema;
      wrapper = {
        package = lib.mkOption {
          description = "Package to wrap";
          type = lib.types.package;
          default = treefmt;
        };
        projectRootFile = lib.mkOption {
          description = ''
            File to look for to determine the root of the project.
          '';
          example = "flake.nix";
        };
      };
      # Outputs
      build = {
        configFile = lib.mkOption {
          description = ''
            Contains the generated config file derived from the settings.
          '';
          type = lib.types.path;
        };
        wrapper = lib.mkOption {
          description = ''
            The treefmt package, wrapped with the config file.
          '';
          type = lib.types.package;
        };
      };
    };
    config.build = {
      configFile = configFormat.generate "treefmt.toml" config.settings;

      wrapper = nixpkgs.writeShellScriptBin "treefmt" ''
        find_up() (
          ancestors=()
          while [[ ! -f ${config.wrapper.projectRootFile} ]]; do
            ancestors+=("$PWD")
            if [[ $PWD == / ]]; then
              echo "ERROR: Unable to locate the projectRootFile (${config.wrapper.projectRootFile}) in any of: ''${ancestors[*]@Q}" >&2
              exit 1
            fi
            cd ..
          done
        )
        tree_root=$(find_up)
        exec ${config.wrapper.package}/bin/treefmt --config-file ${config.build.configFile} "$@" --tree-root "$tree_root"
      '';
    };
  };

  # Use the Nix module system to validate the treefmt config file format.
  evalModule = config:
    lib.evalModules {
      modules = [ module config ];
    };

  # What is used when invoking `nix run github:numtide/treefmt`
  treefmt = rustPackages.rustPlatform.buildRustPackage {
    inherit (cargoToml.package) name version;

    src = builtins.path {
      path = ./.;
      filter = name: type:
        name == toString ./Cargo.toml
        || name == toString ./Cargo.lock
        || lib.hasPrefix (toString ./src) name
        || lib.hasPrefix (toString ./benches) name
      ;
    };

    buildInputs = with nixpkgs; lib.optionals stdenv.isDarwin [ darwin.apple_sdk.frameworks.Security libiconv ];

    doCheck = true;

    cargoLock.lockFile = ./Cargo.lock;

    meta.description = "one CLI to format the code tree";

    passthru.withConfig = config:
      let
        mod = evalModule config;
      in
      mod.config.build.wrapper;
  };

  # Add all the dependencies of treefmt, plus more build tools
  devShell = treefmt.overrideAttrs (prev: {
    shellHook = ''
      # Put the treefmt binary on the PATH when it's built
      export PATH=$PWD/target/debug:$PATH
    '';

    nativeBuildInputs = prev.nativeBuildInputs ++ (with nixpkgs; [
      # Build tools
      rustPackages.clippy
      rust-analyzer

      # Code formatters
      elmPackages.elm-format
      go
      haskellPackages.cabal-fmt
      haskellPackages.ormolu
      mdsh
      nixpkgs-fmt
      nodePackages.prettier
      python3.pkgs.black
      rufo
      rustPackages.rustfmt
      shellcheck
      shfmt
      terraform

      mdbook
    ]);
  });
in
{
  inherit treefmt devShell evalModule module;

  # A collection of packages for the project
  docs = nixpkgs.callPackage ./docs { };

  # Flake attributes
  default = treefmt;
}
