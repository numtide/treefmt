package test

import (
	"os"
	"testing"

	"git.numtide.com/numtide/treefmt/internal/config"

	"github.com/BurntSushi/toml"
	cp "github.com/otiai10/copy"
	"github.com/stretchr/testify/require"
)

func WriteConfig(t *testing.T, path string, cfg config.Config) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create a new config file: %v", err)
	}
	encoder := toml.NewEncoder(f)
	if err = encoder.Encode(cfg); err != nil {
		t.Fatalf("failed to write to config file: %v", err)
	}
}

func TempExamples(t *testing.T) string {
	tempDir := t.TempDir()
	require.NoError(t, cp.Copy("../../test/examples", tempDir), "failed to copy test data to temp dir")
	return tempDir
}

func TempFile(t *testing.T, path string) *os.File {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create temporary file: %v", err)
	}
	return file
}

func RecreateSymlink(t *testing.T, path string) error {
	t.Helper()
	src, err := os.Readlink(path)
	require.NoError(t, err, "failed to read symlink")
	require.NoError(t, os.Remove(path), "failed to remove symlink")
	return os.Symlink(src, path)
}
