package jujutsu

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

const TreeRootCmd = "jj workspace root"

func IsInsideWorktree(path string) (bool, error) {
	// check if the root is a jujutsu repository
	cmd := exec.Command("jj", "workspace", "root")
	cmd.Dir = path

	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && strings.Contains(string(exitErr.Stderr), "There is no jj repo in \".\"") {
			return false, nil
		}

		return false, fmt.Errorf("failed to check if %s is a jujutsu repository: %w", path, err)
	}
	// is a jujutsu repo
	return true, nil
}
