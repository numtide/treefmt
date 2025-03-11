package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func generateShellCompletions(cmd *cobra.Command, args []string) error {
	var err error

	switch args[0] {
	case "bash":
		err = cmd.Root().GenBashCompletion(os.Stdout)
	case "zsh":
		err = cmd.Root().GenZshCompletion(os.Stdout)
	case "fish":
		err = cmd.Root().GenFishCompletion(os.Stdout, true)
	default:
		err = fmt.Errorf("unsupported shell: %s", args[0])
	}

	if err != nil {
		err = fmt.Errorf("failed to generate shell completions: %w", err)
	}

	return err
}
