package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/numtide/treefmt/v2/cmd/format"
	"github.com/numtide/treefmt/v2/stats"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// gitMergetool handles a 3-way merge using `git merge-file` and formats the resulting merged file.
// It expects 4 arguments: current, base, other, and merged filenames.
// Returns an error if the process fails or if arguments are invalid.
func gitMergetool(
	v *viper.Viper,
	statz *stats.Stats,
	cmd *cobra.Command,
	args []string,
) error {
	if len(args) != 4 {
		return fmt.Errorf("expected 4 arguments, got %d", len(args))
	}

	current := args[0]
	base := args[1]
	other := args[2]
	merged := args[3]

	// run treefmt on the first three arguments: current, base and other
	_, _ = fmt.Fprintf(os.Stderr, "formatting: %s, %s, %s\n\n", current, base, other)

	//nolint:wrapcheck
	if err := format.Run(v, statz, cmd, args[:3]); err != nil {
		return err
	}

	// open merge file
	mergeFile, err := os.OpenFile(merged, os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("failed to open merge file: %w", err)
	}

	// merge current base and other
	merge := exec.Command("git", "merge-file", "--stdout", current, base, other)
	_, _ = fmt.Fprintf(os.Stderr, "\n%s\n", merge.String())

	// redirect stdout to the merge file
	merge.Stdout = mergeFile
	// capture stderr
	merge.Stderr = os.Stderr

	if err = merge.Run(); err != nil {
		return fmt.Errorf("failed to run git merge-file: %w", err)
	}

	// close the merge file
	if err = mergeFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary merge file: %w", err)
	}

	// format the merge file
	_, _ = fmt.Fprintf(os.Stderr, "formatting: %s\n\n", mergeFile.Name())

	if err = format.Run(v, stats.New(), cmd, []string{mergeFile.Name()}); err != nil {
		return fmt.Errorf("failed to format merged file: %w", err)
	}

	return nil
}
