package format_test

import (
	"testing"

	"github.com/numtide/treefmt/config"
	"github.com/numtide/treefmt/format"
	"github.com/numtide/treefmt/stats"
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
