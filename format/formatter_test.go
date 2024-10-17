package format_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/numtide/treefmt/config"
	"github.com/numtide/treefmt/format"
	"github.com/numtide/treefmt/stats"
	"github.com/numtide/treefmt/test"
	"github.com/stretchr/testify/require"
)

func TestInvalidFormatterName(t *testing.T) {
	as := require.New(t)

	const batchSize = 1024

	cfg := &config.Config{}
	cfg.OnUnmatched = "info"

	statz := stats.New()

	// simple "empty" config
	_, err := format.NewCompositeFormatter(cfg, &statz, batchSize)
	as.NoError(err)

	// valid name using all the acceptable characters
	cfg.FormatterConfigs = map[string]*config.Formatter{
		"echo_command-1234567890": {
			Command: "echo",
		},
	}

	_, err = format.NewCompositeFormatter(cfg, &statz, batchSize)
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

		_, err = format.NewCompositeFormatter(cfg, &statz, batchSize)
		as.ErrorIs(err, format.ErrInvalidName)
	}
}

func TestFormatterHash(t *testing.T) {
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

	f, err := format.NewCompositeFormatter(cfg, &statz, batchSize)
	as.NoError(err)

	var oldHash string

	oldHash = assertHashChangeAndStable(t, as, cfg, "")

	t.Run("change formatter mod time", func(t *testing.T) {
		for _, name := range []string{"black", "elm-format"} {
			t.Logf("changing mod time of %s", name)

			// tweak mod time
			newTime := time.Now().Add(-time.Minute)
			as.NoError(test.Lutimes(t, filepath.Join(binPath, name), newTime, newTime))

			oldHash = assertHashChangeAndStable(t, as, cfg, oldHash)
		}
	})

	t.Run("modify formatter options", func(_ *testing.T) {
		f, err = format.NewCompositeFormatter(cfg, &statz, batchSize)
		as.NoError(err)

		oldHash = assertHashChangeAndStable(t, as, cfg, "")

		// adjust python includes
		python := cfg.FormatterConfigs["python"]
		python.Includes = []string{"*.py", "*.pyi"}

		newHash, err := f.Hash()
		as.NoError(err)
		as.Equal(oldHash, newHash, "hash should not have changed")

		// adjust python excludes
		python.Excludes = []string{"*.pyi"}

		newHash, err = f.Hash()
		as.NoError(err)
		as.Equal(oldHash, newHash, "hash should not have changed")

		// adjust python options
		python.Options = []string{"-w", "-s"}
		oldHash = assertHashChangeAndStable(t, as, cfg, oldHash)

		// adjust python priority
		python.Priority = 100
		oldHash = assertHashChangeAndStable(t, as, cfg, oldHash)

		// adjust command
		python.Command = "deadnix"
		oldHash = assertHashChangeAndStable(t, as, cfg, oldHash)
	})

	t.Run("add/remove formatters", func(_ *testing.T) {
		cfg.FormatterConfigs["go"] = &config.Formatter{
			Command:  "gofmt",
			Options:  []string{"-w"},
			Includes: []string{"*.go"},
		}

		oldHash = assertHashChangeAndStable(t, as, cfg, oldHash)

		// remove python formatter
		delete(cfg.FormatterConfigs, "python")
		oldHash = assertHashChangeAndStable(t, as, cfg, oldHash)

		// remove elm formatter
		delete(cfg.FormatterConfigs, "elm")
		oldHash = assertHashChangeAndStable(t, as, cfg, oldHash)
	})
}

func assertHashChangeAndStable(
	t *testing.T,
	as *require.Assertions,
	cfg *config.Config,
	oldHash string,
) (h string) {
	t.Helper()

	statz := stats.New()
	f, err := format.NewCompositeFormatter(cfg, &statz, 1024)
	as.NoError(err)

	newHash, err := f.Hash()
	as.NoError(err)
	as.NotEqual(oldHash, newHash, "hash should have changed")

	sameHash, err := f.Hash()
	as.NoError(err)
	as.Equal(newHash, sameHash, "hash should not have changed")

	return newHash
}
