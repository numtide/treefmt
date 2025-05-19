package git

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

const TreeRootCmd = "git rev-parse --show-toplevel"

func IsInsideWorktree(path string) (bool, error) {
	// check if the root is a git repository
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = path

	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && strings.Contains(string(exitErr.Stderr), "not a git repository") {
			return false, nil
		}

		return false, fmt.Errorf("failed to check if %s is a git repository: %w", path, err)
	}

	if strings.Trim(string(out), "\n") != "true" {
		// not a git repo
		return false, nil
	}

	// is a git repo
	return true, nil
}
