package walk_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"testing"
	"time"

	"github.com/numtide/treefmt/v2/stats"
	"github.com/numtide/treefmt/v2/test"
	"github.com/numtide/treefmt/v2/walk"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

//nolint:gochecknoglobals
var sourceExample = `
package main

import "fmt"

func main() {
	fmt.Println("Hello, world!")
}`

func TestWatchReader(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)
	statz := stats.New()

	r, err := walk.NewWatchReader(tempDir, "", &statz, 1024)
	as.NoError(err)

	eg := errgroup.Group{}
	for _, example := range test.ExamplesPaths {
		eg.Go(func() error {
			filePath := path.Join(tempDir, example)

			content, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
			}

			return os.WriteFile(filePath, content, 0o600)
		})
	}

	count := 0

	for count < len(test.ExamplesPaths) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)

		files := make([]*walk.File, 8)
		n, err := r.Read(ctx, files)

		count += n

		cancel()

		if errors.Is(err, io.EOF) {
			break
		}
	}

	as.NoError(eg.Wait())

	as.Equal(40, count)
	as.Equal(40, statz.Value(stats.Traversed))
	as.Equal(0, statz.Value(stats.Matched))
	as.Equal(0, statz.Value(stats.Formatted))
	as.Equal(0, statz.Value(stats.Changed))
}

func TestWatchReaderCreate(t *testing.T) {
	as := require.New(t)

	tempDir := t.TempDir()
	statz := stats.New()

	r, err := walk.NewWatchReader(tempDir, "", &statz, 1024)
	as.NoError(err)

	as.NoError(
		os.WriteFile(
			path.Join(tempDir, "main.go"),
			[]byte(sourceExample),
			0o600,
		),
	)

	count := 0

	for count < 1 {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)

		files := make([]*walk.File, 8)
		n, err := r.Read(ctx, files)

		count += n

		cancel()

		if errors.Is(err, io.EOF) {
			break
		}
	}

	as.Equal(1, count)
	as.Equal(1, statz.Value(stats.Traversed))
	as.Equal(0, statz.Value(stats.Matched))
	as.Equal(0, statz.Value(stats.Formatted))
	as.Equal(0, statz.Value(stats.Changed))
}
