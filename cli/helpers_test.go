package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"git.numtide.com/numtide/treefmt/test"

	"github.com/alecthomas/kong"
	"github.com/stretchr/testify/require"
)

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
	require.NoError(t, err)
	return parser
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
	tempOut := test.TempFile(t, filepath.Join(tempDir, "combined_output"))

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
		return nil, fmt.Errorf("%w: failed to reset temp output for reading", err)
	}

	out, err := io.ReadAll(tempOut)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to read temp output", err)
	}

	// swap outputs back
	os.Stdout = stdout
	os.Stderr = stderr

	return out, nil
}
