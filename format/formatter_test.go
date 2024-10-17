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

	// hash for the first time
	h, err := f.Hash()
	as.NoError(err)

	// hash again without making changes
	h2, err := f.Hash()
	as.NoError(err)
	as.Equal(h, h2, "hash should not have changed")

	t.Run("change formatter mod time", func(t *testing.T) {
		for _, name := range binaries {
			// tweak mod time
			newTime := time.Now().Add(-time.Minute)
			as.NoError(test.Lutimes(t, filepath.Join(binPath, name), newTime, newTime))

			h3, err := f.Hash()
			as.NoError(err)
			as.NotEqual(h2, h3, "hash should have changed")

			// hash again without making changes
			h4, err := f.Hash()
			as.NoError(err)
			as.Equal(h3, h4, "hash should not have changed")
		}
	})

	t.Run("modify formatter options", func(_ *testing.T) {
		f, err = format.NewCompositeFormatter(cfg, &statz, batchSize)
		as.NoError(err)

		h3, err := f.Hash()
		as.NoError(err)

		// adjust python includes
		python := cfg.FormatterConfigs["python"]
		python.Includes = []string{"*.py", "*.pyi"}

		f, err = format.NewCompositeFormatter(cfg, &statz, batchSize)
		as.NoError(err)

		h4, err := f.Hash()
		as.NoError(err)
		as.NotEqual(h3, h4, "hash should have changed")

		// hash again without making changes
		h5, err := f.Hash()
		as.NoError(err)
		as.Equal(h4, h5, "hash should not have changed")

		// adjust python excludes
		python.Excludes = []string{"*.pyi"}

		f, err = format.NewCompositeFormatter(cfg, &statz, batchSize)
		as.NoError(err)

		h6, err := f.Hash()
		as.NoError(err)
		as.NotEqual(h5, h6, "hash should have changed")

		// hash again without making changes
		h7, err := f.Hash()
		as.NoError(err)
		as.Equal(h6, h7, "hash should not have changed")

		// adjust python options
		python.Options = []string{"-w", "-s"}

		f, err = format.NewCompositeFormatter(cfg, &statz, batchSize)
		as.NoError(err)

		h8, err := f.Hash()
		as.NoError(err)
		as.NotEqual(h7, h8, "hash should have changed")

		// hash again without making changes
		h9, err := f.Hash()
		as.NoError(err)
		as.Equal(h8, h9, "hash should not have changed")

		// adjust python priority
		python.Priority = 100

		f, err = format.NewCompositeFormatter(cfg, &statz, batchSize)
		as.NoError(err)

		h10, err := f.Hash()
		as.NoError(err)
		as.NotEqual(h9, h10, "hash should have changed")

		// hash again without making changes
		h11, err := f.Hash()
		as.NoError(err)
		as.Equal(h10, h11, "hash should not have changed")
	})

	t.Run("add/remove formatters", func(_ *testing.T) {
		cfg.FormatterConfigs["go"] = &config.Formatter{
			Command:  "gofmt",
			Options:  []string{"-w"},
			Includes: []string{"*.go"},
		}

		f, err = format.NewCompositeFormatter(cfg, &statz, batchSize)
		as.NoError(err)

		h3, err := f.Hash()
		as.NoError(err)
		as.NotEqual(h2, h3, "hash should have changed")

		// remove python formatter
		delete(cfg.FormatterConfigs, "python")

		f, err = format.NewCompositeFormatter(cfg, &statz, batchSize)
		as.NoError(err)

		h4, err := f.Hash()
		as.NoError(err)
		as.NotEqual(h3, h4, "hash should have changed")

		// hash again without making changes
		h5, err := f.Hash()
		as.NoError(err)
		as.Equal(h4, h5, "hash should not have changed")

		// remove elm formatter
		delete(cfg.FormatterConfigs, "elm")

		f, err = format.NewCompositeFormatter(cfg, &statz, batchSize)
		as.NoError(err)

		h6, err := f.Hash()
		as.NoError(err)
		as.NotEqual(h5, h6, "hash should have changed")

		// hash again without making changes
		h7, err := f.Hash()
		as.NoError(err)
		as.Equal(h6, h7, "hash should not have changed")
	})

	t.Run("modify global excludes", func(_ *testing.T) {
		cfg.Excludes = []string{"*.go"}

		f, err = format.NewCompositeFormatter(cfg, &statz, batchSize)
		as.NoError(err)

		h3, err := f.Hash()
		as.NoError(err)

		cfg.Excludes = []string{"*.go", "*.hs"}

		f, err = format.NewCompositeFormatter(cfg, &statz, batchSize)
		as.NoError(err)

		h4, err := f.Hash()
		as.NoError(err)
		as.NotEqual(h3, h4, "hash should have changed")

		// hash again without making changes
		h5, err := f.Hash()
		as.NoError(err)
		as.Equal(h4, h5, "hash should not have changed")

		// test deprecated global excludes
		cfg.Excludes = nil
		cfg.Global.Excludes = []string{"*.go", "*.hs"}

		f, err = format.NewCompositeFormatter(cfg, &statz, batchSize)
		as.NoError(err)

		h6, err := f.Hash()
		as.NoError(err)
		as.Equal(h4, h6, "Global.Excludes should produce same hash with same values as Excludes")

		// hash again without making changes
		h7, err := f.Hash()
		as.NoError(err)
		as.Equal(h6, h7, "hash should not have changed")
	})
}
