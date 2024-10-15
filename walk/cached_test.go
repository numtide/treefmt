package walk_test

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/numtide/treefmt/stats"
	"github.com/numtide/treefmt/test"
	"github.com/numtide/treefmt/walk"
	"github.com/numtide/treefmt/walk/cache"
	"github.com/stretchr/testify/require"
)

func TestCachedReader(t *testing.T) {
	as := require.New(t)

	batchSize := 1024
	tempDir := test.TempExamples(t)

	readAll := func(path string) (totalCount, newCount, changeCount int, statz stats.Stats) {
		statz = stats.New()

		db, err := cache.Open(tempDir)
		as.NoError(err)
		defer db.Close()

		delegate := walk.NewFilesystemReader(tempDir, path, &statz, batchSize)
		reader, err := walk.NewCachedReader(db, batchSize, delegate)
		as.NoError(err)

		for {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)

			files := make([]*walk.File, 8)
			n, err := reader.Read(ctx, files)

			totalCount += n

			for idx := 0; idx < n; idx++ {
				file := files[idx]

				if file.Cache == nil {
					newCount++
				} else if file.Cache.HasChanged(file.Info) {
					changeCount++
				}

				as.NoError(file.Release(nil))
			}

			cancel()

			if errors.Is(err, io.EOF) {
				break
			}
		}

		as.NoError(reader.Close())

		return totalCount, newCount, changeCount, statz
	}

	totalCount, newCount, changeCount, _ := readAll("")
	as.Equal(32, totalCount)
	as.Equal(32, newCount)
	as.Equal(0, changeCount)

	// read again, should be no changes
	totalCount, newCount, changeCount, _ = readAll("")
	as.Equal(32, totalCount)
	as.Equal(0, newCount)
	as.Equal(0, changeCount)

	// change mod times on some files and try again
	// we subtract a second to account for the 1 second granularity of modtime according to POSIX
	modTime := time.Now().Add(-1 * time.Second)

	as.NoError(os.Chtimes(filepath.Join(tempDir, "treefmt.toml"), time.Now(), modTime))
	as.NoError(os.Chtimes(filepath.Join(tempDir, "shell/foo.sh"), time.Now(), modTime))
	as.NoError(os.Chtimes(filepath.Join(tempDir, "haskell/Nested/Foo.hs"), time.Now(), modTime))

	totalCount, newCount, changeCount, _ = readAll("")
	as.Equal(32, totalCount)
	as.Equal(0, newCount)
	as.Equal(3, changeCount)

	// create some files and try again
	_, err := os.Create(filepath.Join(tempDir, "new.txt"))
	as.NoError(err)

	_, err = os.Create(filepath.Join(tempDir, "fizz.go"))
	as.NoError(err)

	totalCount, newCount, changeCount, _ = readAll("")
	as.Equal(34, totalCount)
	as.Equal(2, newCount)
	as.Equal(0, changeCount)

	// modify some files
	f, err := os.OpenFile(filepath.Join(tempDir, "new.txt"), os.O_WRONLY, 0o644)
	as.NoError(err)
	_, err = f.Write([]byte("foo"))
	as.NoError(err)
	as.NoError(f.Close())

	f, err = os.OpenFile(filepath.Join(tempDir, "fizz.go"), os.O_WRONLY, 0o644)
	as.NoError(err)
	_, err = f.Write([]byte("bla"))
	as.NoError(err)
	as.NoError(f.Close())

	totalCount, newCount, changeCount, _ = readAll("")
	as.Equal(34, totalCount)
	as.Equal(0, newCount)
	as.Equal(2, changeCount)

	// read some paths within the root
	totalCount, newCount, changeCount, _ = readAll("go")
	as.Equal(2, totalCount)
	as.Equal(0, newCount)
	as.Equal(0, changeCount)

	totalCount, newCount, changeCount, _ = readAll("elm/src")
	as.Equal(1, totalCount)
	as.Equal(0, newCount)
	as.Equal(0, changeCount)

	totalCount, newCount, changeCount, _ = readAll("haskell")
	as.Equal(7, totalCount)
	as.Equal(0, newCount)
	as.Equal(0, changeCount)
}
