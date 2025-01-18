package walk

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/numtide/treefmt/v2/stats"
	"golang.org/x/sync/errgroup"
)

type WatchReader struct {
	root string
	path string

	log   *log.Logger
	stats *stats.Stats

	eg      *errgroup.Group
	watcher *fsnotify.Watcher
}

func (f *WatchReader) Read(ctx context.Context, files []*File) (n int, err error) {
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
			err = ctx.Err()
			if err == nil {
				return n, fmt.Errorf("context cancelled: %w", ctx.Err())
			}

			return n, nil

		// read the next event from the channel
		case event, ok := <-f.watcher.Events:
			if !ok {
				// channel was closed, exit the loop
				err = io.EOF

				break LOOP
			}

			// skip if the event is a chmod event since it doesn't change the
			// file contents
			if event.Has(fsnotify.Chmod) {
				continue
			}

			// determine the absolute path since fsnotify only provides the
			// relative path relative to the path we're watching
			path := filepath.Clean(filepath.Join(f.root, f.path, event.Name))

			file, err := os.Open(path)
			if err != nil {
				return n, fmt.Errorf("failed to stat file %s: %w", event.Name, err)
			}
			defer file.Close()
			info, err := file.Stat()
			if err != nil {
				return n, fmt.Errorf("failed to stat file %s: %w", event.Name, err)
			}

			// determine a path relative to the root
			relPath, err := filepath.Rel(f.root, path)
			if err != nil {
				return n, fmt.Errorf("failed to determine a relative path for %s: %w", path, err)
			}

			// add to the file array and increment n
			files[n] = &File{
				Path:    path,
				RelPath: relPath,
				Info:    info,
			}
			n++

		case err, ok := <-f.watcher.Errors:
			if !ok {
				return n, fmt.Errorf("failed to read from watcher: %w", err)
			}
			f.log.Printf("error: %s", err)
		}
	}

	return n, err
}

// Close waits for all watcher processing to complete.
func (f *WatchReader) Close() error {
	err := f.watcher.Close()
	if err != nil {
		return fmt.Errorf("failed to close watcher: %w", err)
	}

	err = f.eg.Wait()
	if err != nil {
		return fmt.Errorf("failed to wait for processing to complete: %w", err)
	}

	return nil
}

func NewWatchReader(
	root string,
	path string,
	statz *stats.Stats,
) (*WatchReader, error) {
	// create an error group for managing the processing loop
	eg := errgroup.Group{}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("failed to create watcher: %v", err)
	}

	r := WatchReader{
		root:    root,
		path:    path,
		log:     log.Default(),
		stats:   statz,
		eg:      &eg,
		watcher: watcher,
	}

	// path is relative to the root, so we create a fully qualified version
	// we also clean the path up in case there are any ../../ components etc.
	fqPath := filepath.Clean(filepath.Join(root, path))

	// ensure the path is within the root
	if !strings.HasPrefix(fqPath, root) {
		return nil, fmt.Errorf("path '%s' is outside of the root '%s'", fqPath, root)
	}

	// start watching the path
	if err := watcher.Add(fqPath); err != nil {
		return nil, fmt.Errorf("failed to watch path %s: %w", fqPath, err)
	}

	return &r, nil
}
