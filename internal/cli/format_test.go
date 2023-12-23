package cli

import (
	"os"
	"testing"

	"git.numtide.com/numtide/treefmt/internal/format"
	"github.com/BurntSushi/toml"
	"github.com/alecthomas/kong"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeConfig(t *testing.T, path string, cfg format.Config) {
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

func newKong(t *testing.T, cli interface{}, options ...kong.Option) *kong.Kong {
	t.Helper()
	options = append([]kong.Option{
		kong.Name("test"),
		kong.Exit(func(int) {
			t.Helper()
			t.Fatalf("unexpected exit()")
		}),
	}, options...)
	parser, err := kong.New(cli, options...)
	assert.NoError(t, err)
	return parser
}

func newCli(t *testing.T, args ...string) (*kong.Context, error) {
	t.Helper()
	p := newKong(t, &Cli)
	return p.Parse(args)
}

func TestAllowMissingFormatter(t *testing.T) {
	as := require.New(t)

	tempDir := t.TempDir()
	configPath := tempDir + "/treefmt.toml"

	writeConfig(t, configPath, format.Config{
		Formatters: map[string]*format.Formatter{
			"foo-fmt": {
				Command: "foo-fmt",
			},
		},
	})

	ctx, err := newCli(t, "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	as.Error(ctx.Run(), format.ErrFormatterNotFound)

	ctx, err = newCli(t, "--config-file", configPath, "--tree-root", tempDir, "--allow-missing-formatter")
	as.NoError(err)

	as.NoError(ctx.Run())
}
