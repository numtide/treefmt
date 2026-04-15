package walk

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/numtide/treefmt/v2/stats"
	"golang.org/x/sync/errgroup"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
)

type CustomReader struct {
	root string
	path string
	cfg  CustomConfig

	log   *log.Logger
	stats *stats.Stats

	eg      *errgroup.Group
	scanner *bufio.Scanner
}

func (c *CustomReader) Read(ctx context.Context, files []*File) (n int, err error) {
	// ensure we record how many files we traversed
	defer func() {
		c.stats.Add(stats.Traversed, n)
	}()

LOOP:
	for n < len(files) {
		select {
		// exit early if the context was cancelled
		case <-ctx.Done():
			return n, ctx.Err() //nolint:wrapcheck

		default:
			if !c.scanner.Scan() {
				if err := c.scanner.Err(); err != nil {
					return n, fmt.Errorf("failed to read custom walker output: %w", err)
				}

				err = io.EOF

				break LOOP
			}

			file, ok, err := c.file(c.scanner.Text())
			if err != nil {
				return n, err
			} else if !ok {
				continue
			}

			files[n] = file
			n++
		}
	}

	return n, err
}

func (c *CustomReader) file(line string) (*File, bool, error) {
	line = strings.TrimSuffix(line, "\r")
	if line == "" {
		return nil, false, nil
	}

	relPath, err := c.relativePath(line)
	if err != nil {
		return nil, false, err
	}

	if !containsPath(c.path, relPath) {
		return nil, false, nil
	}

	path := filepath.Join(c.root, relPath)

	c.log.Debugf("processing file: %s", path)

	info, err := os.Lstat(path)
	switch {
	case os.IsNotExist(err):
		c.log.Warnf(
			"Path %s was emitted by custom walker %s but appears to have been removed from the filesystem",
			path, c.cfg.Name,
		)

		return nil, false, nil
	case err != nil:
		return nil, false, fmt.Errorf("failed to stat %s: %w", path, err)
	case info.IsDir() || info.Mode()&os.ModeSymlink == os.ModeSymlink:
		return nil, false, nil
	}

	return &File{
		Path:    path,
		RelPath: relPath,
		Info:    info,
	}, true, nil
}

func (c *CustomReader) relativePath(entry string) (string, error) {
	entry = filepath.Clean(entry)
	if filepath.IsAbs(entry) {
		relPath, err := filepath.Rel(c.root, entry)
		if err != nil {
			return "", fmt.Errorf("failed to determine a relative path for %s: %w", entry, err)
		}

		entry = relPath
	}

	if entry == ".." || strings.HasPrefix(entry, ".."+string(os.PathSeparator)) || filepath.IsAbs(entry) {
		return "", fmt.Errorf("custom walker %s emitted path %s outside the tree root %s", c.cfg.Name, entry, c.root)
	}

	return entry, nil
}

func (c *CustomReader) Close() error {
	err := c.eg.Wait()
	if err != nil {
		return fmt.Errorf("failed to wait for custom walker %s command to complete: %w", c.cfg.Name, err)
	}

	return nil
}

func containsPath(root string, path string) bool {
	if root == "" || path == root {
		return true
	}

	relPath, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}

	return relPath != ".." &&
		!strings.HasPrefix(relPath, ".."+string(os.PathSeparator)) &&
		!filepath.IsAbs(relPath)
}

func NewCustomReader(
	root string,
	path string,
	statz *stats.Stats,
	cfg CustomConfig,
) (*CustomReader, error) {
	env := expand.ListEnviron(os.Environ()...)

	executable, err := interp.LookPathDir(root, env, cfg.Command)
	if err != nil {
		return nil, fmt.Errorf("failed to find custom walker %s command %q: %w", cfg.Name, cfg.Command, err)
	}

	eg := &errgroup.Group{}

	cmd := exec.CommandContext(context.Background(), executable, cfg.Options...) //nolint:gosec
	cmd.Dir = root

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe for custom walker %s: %w", cfg.Name, err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe for custom walker %s: %w", cfg.Name, err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start custom walker %s: %w", cfg.Name, err)
	}

	eg.Go(func() error {
		return cmd.Wait() //nolint:wrapcheck
	})

	eg.Go(func() error {
		l := log.WithPrefix("walk | custom | " + cfg.Name + " | stderr")

		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			l.Debugf("%s", scanner.Text())
		}

		if err := scanner.Err(); err != nil {
			return fmt.Errorf("failed to read custom walker %s stderr: %w", cfg.Name, err)
		}

		return nil
	})

	return &CustomReader{
		root:    root,
		path:    path,
		cfg:     cfg,
		log:     log.WithPrefix("walk | custom | " + cfg.Name),
		stats:   statz,
		eg:      eg,
		scanner: bufio.NewScanner(stdout),
	}, nil
}
