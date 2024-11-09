package format //nolint:testpackage

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/numtide/treefmt/v2/config"
	"github.com/numtide/treefmt/v2/stats"
	"github.com/numtide/treefmt/v2/test"
	"github.com/stretchr/testify/require"
)

func TestInvalidFormatterName(t *testing.T) {
	as := require.New(t)

	const batchSize = 1024

	cfg := &config.Config{}
	cfg.OnUnmatched = "info"

	statz := stats.New()

	// simple "empty" config
	_, err := NewCompositeFormatter(cfg, &statz, batchSize)
	as.NoError(err)

	// valid name using all the acceptable characters
	cfg.FormatterConfigs = map[string]*config.Formatter{
		"echo_command-1234567890": {
			Command:  "echo",
			Includes: []string{"*"},
		},
	}

	_, err = NewCompositeFormatter(cfg, &statz, batchSize)
	as.NoError(err)

	// test with some bad examples
	for _, character := range []string{
		" ", ":", "?", "*", "[", "]", "(", ")", "|", "&", "<", ">", "\\", "/", "%", "$", "#", "@", "`", "'",
	} {
		cfg.FormatterConfigs = map[string]*config.Formatter{
			"touch_" + character: {
				Command: "touch",
			},
		}

		_, err = NewCompositeFormatter(cfg, &statz, batchSize)
		as.ErrorIs(err, ErrInvalidName)
	}
}

func TestFormatSignature(t *testing.T) {
	as := require.New(t)

	const batchSize = 1024

	statz := stats.New()

	tempDir := t.TempDir()

	// symlink some formatters into temp dir, so we can mess with their mod times
	binPath := filepath.Join(tempDir, "bin")
	as.NoError(os.Mkdir(binPath, 0o755))

	binaries := []string{"black", "elm-format", "gofmt"}

	for _, name := range binaries {
		src, err := exec.LookPath(name)
		as.NoError(err)
		as.NoError(os.Symlink(src, filepath.Join(binPath, name)))
	}

	// prepend our test bin directory to PATH
	t.Setenv("PATH", binPath+":"+os.Getenv("PATH"))

	// start with 2 formatters
	cfg := &config.Config{
		OnUnmatched: "info",
		FormatterConfigs: map[string]*config.Formatter{
			"python": {
				Command:  "black",
				Includes: []string{"*.py"},
			},
			"elm": {
				Command:  "elm-format",
				Options:  []string{"--yes"},
				Includes: []string{"*.elm"},
			},
		},
	}

	oldSignature := assertSignatureChangedAndStable(t, as, cfg, nil)

	t.Run("change formatter mod time", func(t *testing.T) {
		for _, name := range []string{"black", "elm-format"} {
			t.Logf("changing mod time of %s", name)

			// tweak mod time
			newTime := time.Now().Add(-time.Minute)
			as.NoError(test.Lutimes(t, filepath.Join(binPath, name), newTime, newTime))

			oldSignature = assertSignatureChangedAndStable(t, as, cfg, oldSignature)
		}
	})

	t.Run("modify formatter options", func(_ *testing.T) {
		f, err := NewCompositeFormatter(cfg, &statz, batchSize)
		as.NoError(err)

		oldSignature = assertSignatureChangedAndStable(t, as, cfg, nil)

		// adjust python includes
		python := cfg.FormatterConfigs["python"]
		python.Includes = []string{"*.py", "*.pyi"}

		newHash, err := f.signature()
		as.NoError(err)
		as.Equal(oldSignature, newHash, "hash should not have changed")

		// adjust python excludes
		python.Excludes = []string{"*.pyi"}

		newHash, err = f.signature()
		as.NoError(err)
		as.Equal(oldSignature, newHash, "hash should not have changed")

		// adjust python options
		python.Options = []string{"-w", "-s"}
		oldSignature = assertSignatureChangedAndStable(t, as, cfg, oldSignature)

		// adjust python priority
		python.Priority = 100
		oldSignature = assertSignatureChangedAndStable(t, as, cfg, oldSignature)

		// adjust command
		python.Command = "deadnix"
		oldSignature = assertSignatureChangedAndStable(t, as, cfg, oldSignature)
	})

	t.Run("add/remove formatters", func(_ *testing.T) {
		cfg.FormatterConfigs["go"] = &config.Formatter{
			Command:  "gofmt",
			Options:  []string{"-w"},
			Includes: []string{"*.go"},
		}

		oldSignature = assertSignatureChangedAndStable(t, as, cfg, oldSignature)

		// remove python formatter
		delete(cfg.FormatterConfigs, "python")
		oldSignature = assertSignatureChangedAndStable(t, as, cfg, oldSignature)

		// remove elm formatter
		delete(cfg.FormatterConfigs, "elm")
		oldSignature = assertSignatureChangedAndStable(t, as, cfg, oldSignature)
	})
}

func assertSignatureChangedAndStable(
	t *testing.T,
	as *require.Assertions,
	cfg *config.Config,
	oldSignature signature,
) (h signature) {
	t.Helper()

	statz := stats.New()
	f, err := NewCompositeFormatter(cfg, &statz, 1024)
	as.NoError(err)

	newHash, err := f.signature()
	as.NoError(err)
	as.NotEqual(oldSignature, newHash, "hash should have changed")

	sameHash, err := f.signature()
	as.NoError(err)
	as.Equal(newHash, sameHash, "hash should not have changed")

	return newHash
}
