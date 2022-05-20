{ lib, nixpkgs, treefmt, ... }:
let
  # A new kind of option type that calls lib.getExe on derivations
  exeType = lib.mkOptionType {
    name = "exe";
    description = "Path to executable";
    check = (x: lib.isString x || builtins.isPath x || lib.isDerivation x);
    merge = loc: defs:
      let res = lib.mergeOneOption loc defs; in
      if lib.isString res || builtins.isPath res then
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
in
{
  # Schema
  options = {
    settings = configSchema;

    package = lib.mkOption {
      description = "Package wrapped in the build.wrapper output";
      type = lib.types.package;
      default = treefmt;
    };

    projectRootFile = lib.mkOption {
      description = ''
        File to look for to determine the root of the project in the
        build.wrapper.
      '';
      example = "flake.nix";
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
  # Config
  config.build = {
    configFile = configFormat.generate "treefmt.toml" config.settings;

    wrapper = nixpkgs.writeShellScriptBin "treefmt" ''
      find_up() (
        ancestors=()
        while [[ ! -f "$1" ]]; do
          ancestors+=("$PWD")
          if [[ $PWD == / ]]; then
            echo "ERROR: Unable to locate the projectRootFile ($1) in any of: ''${ancestors[*]@Q}" >&2
            exit 1
          fi
          cd ..
        done
      )
      tree_root=$(find_up "${config.projectRootFile}")
      exec ${config.package}/bin/treefmt --config-file ${config.build.configFile} "$@" --tree-root "$tree_root"
    '';
  };
};
