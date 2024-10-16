package walk

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/numtide/treefmt/stats"
	"golang.org/x/sync/errgroup"
)

// FilesystemReader traverses and reads files from a specified root directory and its subdirectories.
type FilesystemReader struct {
	log       *log.Logger
	root      string
	path      string
	batchSize int

	eg *errgroup.Group

	stats   *stats.Stats
	filesCh chan *File
}

// process traverses the filesystem based on the specified paths, queuing files for the next read.
func (f *FilesystemReader) process() error {
	// ensure filesCh is closed on return
	defer func() {
		close(f.filesCh)
	}()

	// f.path is relative to the root, so we create a fully qualified version
	// we also clean the path up in case there are any ../../ components etc.
	path := filepath.Clean(filepath.Join(f.root, f.path))

	// ensure the path is within the root
	if !strings.HasPrefix(path, f.root) {
		return fmt.Errorf("path '%s' is outside of the root '%s'", path, f.root)
	}

	// walk the path
	return filepath.Walk(path, func(path string, info fs.FileInfo, err error) error {
		// return errors immediately
		if err != nil {
			return err
		}

		// ignore directories and symlinks
		if info.IsDir() || info.Mode()&os.ModeSymlink == os.ModeSymlink {
			return nil
		}

		// determine a path relative to the root
		relPath, err := filepath.Rel(f.root, path)
		if err != nil {
			return fmt.Errorf("failed to determine a relative path for %s: %w", path, err)
		}

		// create a new file and pass to the files channel
		file := File{
			Path:    path,
			RelPath: relPath,
			Info:    info,
		}

		f.filesCh <- &file

		f.log.Debugf("file queued %s", file.RelPath)

		return nil
	})
}

// Read populates the provided files array with as many files as are available until the provided context is cancelled.
// You must ensure to pass a context with a timeout otherwise this will block until files is full.
func (f *FilesystemReader) Read(ctx context.Context, files []*File) (n int, err error) {
	// ensure we record how many files we traversed
	defer func() {
		f.stats.Add(stats.Traversed, n)
	}()

LOOP:
	// keep filling files up to it's length
	for n < len(files) {
		select {
		// exit early if the context was cancelled
		case <-ctx.Done():
			return n, ctx.Err()

		// read the next file from the channel
		case file, ok := <-f.filesCh:
			if !ok {
				// channel was closed, exit the loop
				err = io.EOF

				break LOOP
			}

			// add to the file array and increment n
			files[n] = file
			n++
		}
	}

	return n, err
}

// Close waits for all filesystem processing to complete.
func (f *FilesystemReader) Close() error {
	return f.eg.Wait()
}

// NewFilesystemReader creates a new instance of FilesystemReader to traverse and read files from the specified paths
// and root.
func NewFilesystemReader(
	root string,
	path string,
	statz *stats.Stats,
	batchSize int,
) *FilesystemReader {
	// create an error group for managing the processing loop
	eg := errgroup.Group{}

	r := FilesystemReader{
		log:       log.WithPrefix("walk[filesystem]"),
		root:      root,
		path:      path,
		batchSize: batchSize,

		eg: &eg,

		stats:   statz,
		filesCh: make(chan *File, batchSize*runtime.NumCPU()),
	}

	// start processing loop
	eg.Go(r.process)

	return &r
}
