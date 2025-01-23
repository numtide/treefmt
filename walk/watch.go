package walk

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/fsnotify/fsnotify"
	"github.com/numtide/treefmt/v2/stats"
)

type WatchReader struct {
	root string
	path string

	log   *log.Logger
	stats *stats.Stats

	watcher *fsnotify.Watcher
}

func (f *WatchReader) Read(ctx context.Context, files []*File) (n int, err error) {
	// ensure we record how many files we traversed
	defer func() {
		f.stats.Add(stats.Traversed, n)
	}()

	// listen for shutdown signal and cancel the context
	exit := make(chan os.Signal, 1)
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)

	for n < len(files) {
		select {
		// since we can't detect exit using the context as watch
		// events are an unbounded channel, we need to check
		// for this explicitly
		case <-exit:
			return n, io.EOF

			// exit early if the context was cancelled
		case <-ctx.Done():
			err = ctx.Err()
			if err == nil {
				return n, fmt.Errorf("context cancelled: %w", ctx.Err())
			}

			return n, fmt.Errorf("context error: %w", err)

		// read the next event from the channel
		case event, ok := <-f.watcher.Events:
			if !ok {
				// channel was closed, exit the loop
				return n, io.EOF
			}

			// skip if the event which doesn't have content changed
			if !event.Has(fsnotify.Write) {
				return n, nil
			}

			file, err := os.Open(event.Name)
			if errors.Is(err, os.ErrNotExist) {
				// file was deleted, skip it
				return n, nil
			} else if err != nil {
				return n, fmt.Errorf("failed to stat file %s: %w", event.Name, err)
			}
			defer file.Close()

			info, err := file.Stat()
			if err != nil {
				return n, fmt.Errorf("failed to stat file %s: %w", event.Name, err)
			}

			// determine a path relative to the root
			relPath, err := filepath.Rel(f.root, event.Name)
			if err != nil {
				return n, fmt.Errorf("failed to determine a relative path for %s: %w", event.Name, err)
			}

			// add to the file array and increment n
			files[n] = &File{
				Path:    event.Name,
				RelPath: relPath,
				Info:    info,
			}
			n++

		case err, ok := <-f.watcher.Errors:
			if !ok {
				return n, fmt.Errorf("failed to read from watcher: %w", err)
			}
		}
	}

	return n, nil
}

// Close waits for all watcher processing to complete.
func (f *WatchReader) Close() error {
	err := f.watcher.Close()
	if err != nil {
		return fmt.Errorf("failed to close watcher: %w", err)
	}

	return nil
}

func NewWatchReader(
	root string,
	path string,
	statz *stats.Stats,
	batchSize uint,
) (*WatchReader, error) {
	watcher, err := fsnotify.NewBufferedWatcher(batchSize)
	if err != nil {
		log.Fatalf("failed to create watcher: %v", err)
	}

	r := WatchReader{
		root:    root,
		path:    path,
		log:     log.Default(),
		stats:   statz,
		watcher: watcher,
	}

	// path is relative to the root, so we create a fully qualified version
	// we also clean the path up in case there are any ../../ components etc.
	fqPath := filepath.Clean(filepath.Join(root, path))

	// ensure the path is within the root
	if !strings.HasPrefix(fqPath, root) {
		return nil, fmt.Errorf("path '%s' is outside of the root '%s'", fqPath, root)
	}

	// start watching for changes recursively
	err = filepath.Walk(fqPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			err := watcher.Add(path)
			if err != nil {
				return fmt.Errorf("failed to watch path %s: %w", path, err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory %s: %w", fqPath, err)
	}

	return &r, nil
}
