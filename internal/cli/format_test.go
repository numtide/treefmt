package cli

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"git.numtide.com/numtide/treefmt/internal/format"
	"github.com/BurntSushi/toml"
	"github.com/alecthomas/kong"
	"github.com/juju/errors"
	cp "github.com/otiai10/copy"
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

func tempFile(t *testing.T, path string) *os.File {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create temporary file: %v", err)
	}
	return file
}

func cmd(t *testing.T, args ...string) ([]byte, error) {
	t.Helper()

	// create a new kong context
	p := newKong(t, &Cli)
	ctx, err := p.Parse(args)
	if err != nil {
		return nil, err
	}

	tempDir := t.TempDir()
	tempOut := tempFile(t, filepath.Join(tempDir, "combined_output"))

	// capture standard outputs before swapping them
	stdout := os.Stdout
	stderr := os.Stderr

	// swap them temporarily
	os.Stdout = tempOut
	os.Stderr = tempOut

	// run the command
	if err = ctx.Run(); err != nil {
		return nil, err
	}

	// reset and read the temporary output
	if _, err = tempOut.Seek(0, 0); err != nil {
		return nil, errors.Annotate(err, "failed to reset temp output for reading")
	}

	out, err := io.ReadAll(tempOut)
	if err != nil {
		return nil, errors.Annotate(err, "failed to read temp output")
	}

	// swap outputs back
	os.Stdout = stdout
	os.Stderr = stderr

	return out, nil
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

	_, err := cmd(t, "--config-file", configPath, "--tree-root", tempDir)
	as.ErrorIs(err, format.ErrFormatterNotFound)

	_, err = cmd(t, "--config-file", configPath, "--tree-root", tempDir, "--allow-missing-formatter")
	as.NoError(err)
}

func TestSpecifyingFormatters(t *testing.T) {
	as := require.New(t)

	tempDir := t.TempDir()
	configPath := tempDir + "/treefmt.toml"

	as.NoError(cp.Copy("../../test/examples", tempDir), "failed to copy test data to temp dir")

	writeConfig(t, configPath, format.Config{
		Formatters: map[string]*format.Formatter{
			"elm": {
				Command:  "echo",
				Includes: []string{"*.elm"},
			},
			"nix": {
				Command:  "echo",
				Includes: []string{"*.nix"},
			},
			"ruby": {
				Command:  "echo",
				Includes: []string{"*.rb"},
			},
		},
	})

	out, err := cmd(t, "--clear-cache", "--config-file", configPath, "--tree-root", tempDir)
	as.NoError(err)
	as.Contains(string(out), "3 files changed")

	out, err = cmd(t, "--clear-cache", "--config-file", configPath, "--tree-root", tempDir, "--formatters", "elm,nix")
	as.NoError(err)
	as.Contains(string(out), "2 files changed")

	out, err = cmd(t, "--clear-cache", "--config-file", configPath, "--tree-root", tempDir, "--formatters", "ruby,nix")
	as.NoError(err)
	as.Contains(string(out), "2 files changed")

	out, err = cmd(t, "--clear-cache", "--config-file", configPath, "--tree-root", tempDir, "--formatters", "nix")
	as.NoError(err)
	as.Contains(string(out), "1 files changed")

	// test bad names

	out, err = cmd(t, "--clear-cache", "--config-file", configPath, "--tree-root", tempDir, "--formatters", "foo")
	as.Errorf(err, "formatter not found in config: foo")

	out, err = cmd(t, "--clear-cache", "--config-file", configPath, "--tree-root", tempDir, "--formatters", "bar,foo")
	as.Errorf(err, "formatter not found in config: bar")
}
