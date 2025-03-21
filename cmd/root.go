package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/numtide/treefmt/v2/build"
	"github.com/numtide/treefmt/v2/cmd/format"
	_init "github.com/numtide/treefmt/v2/cmd/init"
	"github.com/numtide/treefmt/v2/config"
	"github.com/numtide/treefmt/v2/stats"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewRoot() (*cobra.Command, *stats.Stats) {
	// create a viper instance for reading in config
	v, err := config.NewViper()
	if err != nil {
		cobra.CheckErr(fmt.Errorf("failed to create viper instance: %w", err))
	}

	// create a new stats instance
	statz := stats.New()

	// create out root command
	cmd := &cobra.Command{
		Use:     build.Name + " <paths...>",
		Short:   "The formatter multiplexer",
		Version: build.Version,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runE(v, statz, cmd, args)
		},
	}

	// update version template
	cmd.SetVersionTemplate("treefmt {{.Version}}")

	fs := cmd.Flags()

	// add our config flags to the command's flag set
	config.SetFlags(fs)

	// xor tree-root and tree-root-file flags
	cmd.MarkFlagsMutuallyExclusive("tree-root", "tree-root-file")

	cmd.HelpTemplate()

	// add a config file flag and some others for special subcommands
	fs.String(
		"config-file", "",
		"Load the config file from the given path (defaults to searching upwards for treefmt.toml or "+
			".treefmt.toml).",
	)

	// add a flag for the init sub command
	fs.BoolP(
		"init", "i", false,
		"Create a treefmt.toml file in the current directory.",
	)

	// add a flag for generating shell completions
	fs.String(
		"completion", "",
		"[bash|zsh|fish] Generate shell completion scripts for the specified shell.",
	)

	// add a flag for git merge tool sub command
	fs.Bool("git-mergetool", false, "Use treefmt as a git merge tool. Accepts four arguments: current base other merged.")

	// bind our command's flags to viper
	if err := v.BindPFlags(fs); err != nil {
		cobra.CheckErr(fmt.Errorf("failed to bind global config to viper: %w", err))
	}

	// bind prj_root to the tree-root flag, allowing viper to handle environment override for us
	// conforms with https://github.com/numtide/prj-spec/blob/main/PRJ_SPEC.md
	cobra.CheckErr(v.BindPFlag("prj_root", fs.Lookup("tree-root")))

	return cmd, statz
}

func runE(v *viper.Viper, statz *stats.Stats, cmd *cobra.Command, args []string) error {
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
		if initErr := _init.Run(); initErr != nil {
			return fmt.Errorf("failed to run init command: %w", initErr)
		}

		return nil
	}

	// check if we are running the completion command
	if shell, err := flags.GetString("completion"); err != nil {
		return fmt.Errorf("failed to read completion flag: %w", err)
	} else if shell != "" {
		if completionsErr := generateShellCompletions(cmd, []string{shell}); completionsErr != nil {
			return fmt.Errorf("failed to generate shell completions: %w", completionsErr)
		}

		return nil
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

	filenames := []string{"treefmt.toml", ".treefmt.toml"}

	// look in PRJ_ROOT if set
	if prjRoot := os.Getenv("PRJ_ROOT"); configFile == "" && prjRoot != "" {
		configFile, _ = config.Find(prjRoot, filenames...)
	}

	// search up from the working directory
	if configFile == "" {
		configFile, _, err = config.FindUp(workingDir, filenames...)
	}

	// error out if we couldn't find the config file
	if err != nil {
		cmd.SilenceUsage = true

		return fmt.Errorf("failed to find treefmt config file: %w", err)
	}

	log.Debugf("using config file: %s", configFile)

	// read in the config
	v.SetConfigFile(configFile)

	if err := v.ReadInConfig(); err != nil {
		cobra.CheckErr(fmt.Errorf("failed to read config file '%s': %w", configFile, err))
	}

	// configure logging
	log.SetOutput(os.Stderr)
	log.SetReportTimestamp(false)

	if v.GetBool("quiet") {
		// if quiet, we only log errors
		log.SetLevel(log.ErrorLevel)
	} else {
		// otherwise, the verbose flag controls the log level
		switch v.GetInt("verbose") {
		case 0:
			log.SetLevel(log.WarnLevel)
		case 1:
			log.SetLevel(log.InfoLevel)
		default:
			log.SetLevel(log.DebugLevel)
		}
	}

	// git mergetool
	if merge, err := flags.GetBool("git-mergetool"); err != nil {
		cobra.CheckErr(fmt.Errorf("failed to read git-mergetool flag: %w", err))
	} else if merge {
		return gitMergetool(v, statz, cmd, args)
	}

	// format
	return format.Run(v, statz, cmd, args) //nolint:wrapcheck
}
