package walk

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"charm.land/log/v2"
	"github.com/numtide/treefmt/v2/stats"
	"golang.org/x/sync/errgroup"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
)

type PathStreamConfig struct {
	Name        string
	Command     string
	Options     []string
	PathFilters []string
}

type PathStreamReader struct {
	root string
	cfg  PathStreamConfig

	log   *log.Logger
	stats *stats.Stats

	cancel  context.CancelFunc
	eg      *errgroup.Group
	scanner *bufio.Scanner

	waitMu       sync.Mutex
	waitErr      error
	waitCanceled bool
}

func (p *PathStreamReader) Read(ctx context.Context, files []*File) (n int, err error) {
	// ensure we record how many files we traversed
	defer func() {
		p.stats.Add(stats.Traversed, n)
	}()

	for n < len(files) {
		select {
		// exit early if the context was cancelled
		case <-ctx.Done():
			return n, ctx.Err() //nolint:wrapcheck

		default:
			if !p.scanner.Scan() {
				if err := p.scanner.Err(); err != nil {
					return n, fmt.Errorf("failed to read %s output: %w", p.cfg.Name, err)
				}

				return n, io.EOF
			}

			file, ok, err := p.file(p.scanner.Text())
			if err != nil {
				return n, err
			} else if !ok {
				continue
			}

			files[n] = file
			n++
		}
	}

	return n, nil
}

func (p *PathStreamReader) file(record string) (*File, bool, error) {
	record = strings.TrimSuffix(record, "\r")
	if record == "" {
		return nil, false, nil
	}

	relPath, err := p.relativePath(record)
	if err != nil {
		return nil, false, err
	}

	if !containsAnyPath(p.cfg.PathFilters, relPath) {
		return nil, false, nil
	}

	path := filepath.Join(p.root, relPath)

	p.log.Debugf("processing file: %s", path)

	info, err := os.Lstat(path)
	switch {
	case os.IsNotExist(err):
		p.log.Warnf(
			"Path %s was emitted by %s but no file exists at that path",
			path, p.cfg.Name,
		)

		return nil, false, nil
	case err != nil:
		return nil, false, fmt.Errorf("failed to stat %s: %w", path, err)
	case !info.Mode().IsRegular():
		return nil, false, nil
	}

	return &File{
		Path:    path,
		RelPath: relPath,
		Info:    info,
	}, true, nil
}

func (p *PathStreamReader) relativePath(entry string) (string, error) {
	entry = filepath.Clean(entry)
	if filepath.IsAbs(entry) {
		relPath, err := filepath.Rel(p.root, entry)
		if err != nil {
			return "", fmt.Errorf("failed to determine a relative path for %s: %w", entry, err)
		}

		entry = relPath
	}

	if entry == ".." || strings.HasPrefix(entry, ".."+string(os.PathSeparator)) || filepath.IsAbs(entry) {
		return "", fmt.Errorf("%s emitted path %s outside the tree root %s", p.cfg.Name, entry, p.root)
	}

	return entry, nil
}

func (p *PathStreamReader) Close() error {
	// Unblock the walker command if the caller stopped draining Read() before
	// EOF. Without this, the command can block writing stdout while Close waits
	// for it to exit.
	p.cancel()

	err := p.eg.Wait()
	if err != nil {
		return fmt.Errorf("failed to wait for %s command to complete: %w", p.cfg.Name, err)
	}

	p.waitMu.Lock()
	defer p.waitMu.Unlock()

	if p.waitErr != nil && !errors.Is(p.waitErr, context.Canceled) && !p.waitCanceled {
		return fmt.Errorf("failed to wait for %s command to complete: %w", p.cfg.Name, p.waitErr)
	}

	return nil
}

func splitPathRecord(data []byte, atEOF bool) (advance int, token []byte, err error) {
	for i, b := range data {
		if b == '\n' || b == 0 {
			return i + 1, data[:i], nil
		}
	}

	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}

	return 0, nil, nil
}

func containsAnyPath(filters []string, path string) bool {
	if len(filters) == 0 {
		return true
	}

	for _, filter := range filters {
		if containsPath(filter, path) {
			return true
		}
	}

	return false
}

func containsPath(root string, path string) bool {
	if root == "" || root == "." || path == root {
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

func NewPathStreamReader(
	root string,
	statz *stats.Stats,
	cfg PathStreamConfig,
) (*PathStreamReader, error) {
	env := expand.ListEnviron(os.Environ()...)

	executable, err := interp.LookPathDir(root, env, cfg.Command)
	if err != nil {
		return nil, fmt.Errorf("failed to find %s command %q: %w", cfg.Name, cfg.Command, err)
	}

	eg := &errgroup.Group{}

	args := append([]string{}, cfg.Options...)
	args = append(args, cfg.PathFilters...)

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = root

	// Don't use cmd.StdoutPipe here. Its docs say it is incorrect to call
	// Cmd.Wait before all reads from the pipe have completed, and we wait for
	// the command in a producer goroutine while Read drains stdout.
	// See https://pkg.go.dev/os/exec#Cmd.StdoutPipe.
	stdout, stdoutW := io.Pipe()
	stderr, stderrW := io.Pipe()

	cmd.Stdout = stdoutW
	cmd.Stderr = stderrW

	scanner := bufio.NewScanner(stdout)
	scanner.Split(splitPathRecord)

	reader := &PathStreamReader{
		root:    root,
		cfg:     cfg,
		log:     log.WithPrefix("walk | " + cfg.Name),
		stats:   statz,
		cancel:  cancel,
		eg:      eg,
		scanner: scanner,
	}

	eg.Go(func() error {
		err := cmd.Run()

		reader.waitMu.Lock()
		reader.waitErr = err
		reader.waitCanceled = ctx.Err() != nil
		reader.waitMu.Unlock()

		closeErr := stdoutW.Close()
		if stderrCloseErr := stderrW.Close(); stderrCloseErr != nil && closeErr == nil {
			closeErr = stderrCloseErr
		}

		return closeErr
	})

	eg.Go(func() error {
		l := log.WithPrefix("walk | " + cfg.Name + " | stderr")

		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			l.Debugf("%s", scanner.Text())
		}

		if err := scanner.Err(); err != nil {
			return fmt.Errorf("failed to read %s stderr: %w", cfg.Name, err)
		}

		return nil
	})

	return reader, nil
}
