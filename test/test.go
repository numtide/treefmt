package test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/numtide/treefmt/v2/config"
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
	t.Helper()
	tempDir := t.TempDir()
	TempExamplesInDir(t, tempDir)

	return tempDir
}

func TempExamplesInDir(t *testing.T, dir string) {
	t.Helper()
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
// TODO: this will need to be adapted if we support Windows.
func Lutimes(t *testing.T, path string, atime time.Time, mtime time.Time) error {
	t.Helper()

	var utimes [2]unix.Timeval
	utimes[0] = unix.NsecToTimeval(atime.UnixNano())
	utimes[1] = unix.NsecToTimeval(mtime.UnixNano())

	// Change the timestamps of the path. If it's a symlink, it updates the symlink's timestamps, not the target's.
	err := unix.Lutimes(path, utimes[0:])
	if err != nil {
		return fmt.Errorf("failed to change times: %w", err)
	}

	return nil
}

func LutimesBump(t *testing.T, path string, atime time.Duration, mtime time.Duration) {
	t.Helper()

	now := time.Now()
	newAtime := now.Add(atime)
	newMtime := now.Add(mtime)

	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		return Lutimes(t, path, newAtime, newMtime)
	})
	if err != nil {
		t.Fatalf("failed to bump modtimes: %v", err)
	}
}

// ChangeWorkDir changes the current working directory for the duration of the test.
// The original directory is restored when the test ends.
func ChangeWorkDir(t *testing.T, dir string) {
	t.Helper()

	// capture current cwd, so we can replace it after the test is finished
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(fmt.Errorf("failed to get current working directory: %w", err))
	}

	t.Cleanup(func() {
		// return to the previous working directory
		t.Chdir(cwd)
	})

	// change to the new directory
	t.Chdir(dir)
}
