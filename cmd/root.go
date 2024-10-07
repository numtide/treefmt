package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/numtide/treefmt/build"
	"github.com/numtide/treefmt/cmd/format"
	_init "github.com/numtide/treefmt/cmd/init"
	"github.com/numtide/treefmt/config"
	"github.com/numtide/treefmt/walk"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewRoot() *cobra.Command {
	var (
		treefmtInit bool
		configFile  string
	)

	// create a viper instance for reading in config
	v := config.NewViper()

	// create out root command
	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s <paths...>", build.Name),
		Short:   "One CLI to format your repo",
		Version: build.Version,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runE(v, cmd, args)
		},
	}

	// update version template
	cmd.SetVersionTemplate("treefmt {{.Version}}")

	// add our config flags to the command's flag set
	pfs := config.SetFlags(cmd.Flags())

	// xor tree-root and tree-root-file flags
	cmd.MarkFlagsMutuallyExclusive("tree-root", "tree-root-file")

	// add a couple of special flags which don't have a corresponding entry in treefmt.toml
	pfs.StringVar(
		&configFile, "config-file", "",
		"Load the config file from the given path (defaults to searching upwards for treefmt.toml or .treefmt.toml).",
	)

	pfs.BoolVarP(&treefmtInit, "init", "i", false, "Create a treefmt.toml file in the current directory.")

	// bind prj_root to the tree-root flag, allowing viper to handle environment override for us
	// conforms with https://github.com/numtide/prj-spec/blob/main/PRJ_SPEC.md
	cobra.CheckErr(v.BindPFlag("prj_root", pfs.Lookup("tree-root")))

	// bind our command's flags to viper, ensuring a flag → env → config order of precedence when looking for a value
	if err := v.BindPFlags(pfs); err != nil {
		cobra.CheckErr(fmt.Errorf("failed to bind global config to viper: %w", err))
	}

	return cmd
}

func runE(v *viper.Viper, cmd *cobra.Command, args []string) error {
	flags := cmd.Flags()

	// change working directory if required
	workingDir, err := filepath.Abs(v.GetString("working-dir"))
	if err != nil {
		return fmt.Errorf("failed to get absolute path for working directory: %w", err)
	} else if err = os.Chdir(workingDir); err != nil {
		return fmt.Errorf("failed to change working directory: %w", err)
	}

	// check if we are running the init command
	if init, err := flags.GetBool("init"); err != nil {
		return fmt.Errorf("failed to read init flag: %w", err)
	} else if init {
		return _init.Run()
	}

	// otherwise attempt to load the config file

	// use the path specified by the flag
	configFile, err := flags.GetString("config-file")
	if err != nil {
		return fmt.Errorf("failed to read config-file flag: %w", err)
	}

	// fallback to env
	if configFile == "" {
		configFile = os.Getenv("TREEFMT_CONFIG")
	}

	// find the config file if one was not specified
	if configFile == "" {
		if configFile, _, err = walk.FindUp(workingDir, "treefmt.toml", ".treefmt.toml"); err != nil {
			return fmt.Errorf("failed to find treefmt config file: %w", err)
		}
	}

	// read in the config
	v.SetConfigFile(configFile)

	if err := v.ReadInConfig(); err != nil {
		cobra.CheckErr(fmt.Errorf("failed to read config file '%s': %w", configFile, err))
	}

	// configure logging
	log.SetOutput(os.Stderr)
	log.SetReportTimestamp(false)

	switch v.GetInt("verbose") {
	case 0:
		log.SetLevel(log.WarnLevel)
	case 1:
		log.SetLevel(log.InfoLevel)
	default:
		log.SetLevel(log.DebugLevel)
	}

	// format
	return format.Run(v, cmd, args)
}
