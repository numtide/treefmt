package cli

import (
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/charmbracelet/log"

	"git.numtide.com/numtide/treefmt/stats"

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
	p := newKong(t, New(), NewOptions()...)
	ctx, err := p.Parse(args)
	if err != nil {
		return nil, err
	}

	tempDir := t.TempDir()
	tempOut := test.TempFile(t, tempDir, "combined_output", nil)

	// capture standard outputs before swapping them
	stdout := os.Stdout
	stderr := os.Stderr

	// swap them temporarily
	os.Stdout = tempOut
	os.Stderr = tempOut

	log.SetOutput(tempOut)

	// run the command
	if err = ctx.Run(); err != nil {
		return nil, err
	}

	// reset and read the temporary output
	if _, err = tempOut.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to reset temp output for reading: %w", err)
	}

	out, err := io.ReadAll(tempOut)
	if err != nil {
		return nil, fmt.Errorf("failed to read temp output: %w", err)
	}

	// swap outputs back
	os.Stdout = stdout
	os.Stderr = stderr
	log.SetOutput(stderr)

	return out, nil
}

func assertStats(t *testing.T, as *require.Assertions, traversed int32, emitted int32, matched int32, formatted int32) {
	t.Helper()
	as.Equal(traversed, stats.Value(stats.Traversed), "stats.traversed")
	as.Equal(emitted, stats.Value(stats.Emitted), "stats.emitted")
	as.Equal(matched, stats.Value(stats.Matched), "stats.matched")
	as.Equal(formatted, stats.Value(stats.Formatted), "stats.formatted")
}

func assertFormatted(t *testing.T, as *require.Assertions, output []byte, count int) {
	t.Helper()
	as.Contains(string(output), fmt.Sprintf("(%d changed)", count))
}
