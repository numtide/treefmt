package walk

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/numtide/treefmt/stats"
	"golang.org/x/sync/errgroup"
)

type FilesystemReader struct {
	root      string
	paths     []string
	stats     *stats.Stats
	batchSize int

	log     *log.Logger
	filesCh chan *File

	eg *errgroup.Group
}

func (f *FilesystemReader) process() error {
	defer func() {
		close(f.filesCh)
	}()

	walkFn := func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// ignore directories and symlinks
		if info.IsDir() || info.Mode()&os.ModeSymlink == os.ModeSymlink {
			return nil
		}

		relPath, err := filepath.Rel(f.root, path)
		if err != nil {
			return fmt.Errorf("failed to determine a relative path for %s: %w", path, err)
		}

		file := File{
			Path:    path,
			RelPath: relPath,
			Info:    info,
		}
		f.filesCh <- &file

		f.log.Debugf("file queued %s", file.RelPath)

		return nil
	}

	for idx := range f.paths {
		path := filepath.Clean(filepath.Join(f.root, f.paths[idx]))
		if !strings.HasPrefix(path, f.root) {
			return fmt.Errorf("path '%s' is outside of the root '%s'", path, f.root)
		}

		if err := filepath.Walk(path, walkFn); err != nil {
			return err
		}
	}

	return nil
}

func (f *FilesystemReader) Read(ctx context.Context, files []*File) (n int, err error) {
	idx := 0

LOOP:
	for idx < len(files) {
		select {
		case <-ctx.Done():
			return idx, ctx.Err()
		case file, ok := <-f.filesCh:
			if !ok {
				break LOOP
			}
			files[idx] = file
			idx++
			f.stats.Add(stats.Traversed, 1)
		}
	}

	return idx, nil
}

func (f *FilesystemReader) Close() error {
	return f.eg.Wait()
}

func NewFilesystemReader(
	root string,
	paths []string,
	statz *stats.Stats,
	batchSize int,
) *FilesystemReader {
	if len(paths) == 0 {
		paths = []string{"."}
	}

	eg := errgroup.Group{}
	r := FilesystemReader{
		root:      root,
		paths:     paths,
		stats:     statz,
		log:       log.WithPrefix("walk[filesystem]"),
		batchSize: batchSize,

		filesCh: make(chan *File, batchSize*runtime.NumCPU()),
		eg:      &eg,
	}
	eg.Go(r.process)
	return &r
}
