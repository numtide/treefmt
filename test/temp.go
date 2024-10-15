package test

import (
	"os"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/numtide/treefmt/config"
	cp "github.com/otiai10/copy"
	"github.com/stretchr/testify/require"
)

func WriteConfig(t *testing.T, path string, cfg *config.Config) {
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
	TempExamplesInDir(t, tempDir)

	return tempDir
}

func TempExamplesInDir(t *testing.T, dir string) {
	require.NoError(t, cp.Copy("../test/examples", dir), "failed to copy test data to dir")

	// we have second precision mod time tracking, so we wait a second before returning, so we don't trigger false
	// positives for things like fail on change
	time.Sleep(time.Second)
}

func TempFile(t *testing.T, dir string, pattern string, contents *string) *os.File {
	t.Helper()

	file, err := os.CreateTemp(dir, pattern)
	require.NoError(t, err, "failed to create temp file")

	if contents == nil {
		return file
	}

	_, err = file.WriteString(*contents)
	require.NoError(t, err, "failed to write contents to temp file")
	require.NoError(t, file.Close(), "failed to close temp file")

	file, err = os.Open(file.Name())
	require.NoError(t, err, "failed to open temp file")

	return file
}

func RecreateSymlink(t *testing.T, path string) error {
	t.Helper()

	src, err := os.Readlink(path)

	require.NoError(t, err, "failed to read symlink")
	require.NoError(t, os.Remove(path), "failed to remove symlink")

	return os.Symlink(src, path)
}
