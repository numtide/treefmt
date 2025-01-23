package walk_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/numtide/treefmt/v2/stats"
	"github.com/numtide/treefmt/v2/test"
	"github.com/numtide/treefmt/v2/walk"
	"github.com/stretchr/testify/require"
)

func TestFilesystemReader(t *testing.T) {
	as := require.New(t)

	tempDir := test.TempExamples(t)
	statz := stats.New()

	r := walk.NewFilesystemReader(tempDir, "", &statz, 1024)

	count := 0

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)

		files := make([]*walk.File, 8)
		n, err := r.Read(ctx, files)

		for i := count; i < count+n; i++ {
			as.Equal(test.ExamplesPaths[i], files[i-count].RelPath)
		}

		count += n

		cancel()

		if errors.Is(err, io.EOF) {
			break
		}
	}

	as.Equal(33, count)
	as.Equal(33, statz.Value(stats.Traversed))
	as.Equal(0, statz.Value(stats.Matched))
	as.Equal(0, statz.Value(stats.Formatted))
	as.Equal(0, statz.Value(stats.Changed))
}
