package test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/numtide/treefmt/config"
	cp "github.com/otiai10/copy"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
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

// Lutimes is a convenience wrapper for using unix.Lutimes
// TODO: this will need adapted if we support Windows.
func Lutimes(t *testing.T, path string, atime time.Time, mtime time.Time) error {
	t.Helper()

	var utimes [2]unix.Timeval
	utimes[0] = unix.NsecToTimeval(atime.UnixNano())
	utimes[1] = unix.NsecToTimeval(mtime.UnixNano())

	// Change the timestamps of the symlink itself
	err := unix.Lutimes(path, utimes[0:])
	if err != nil {
		return fmt.Errorf("failed to change times: %w", err)
	}

	return nil
}
