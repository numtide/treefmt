package walk

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

//go:generate enumer -type=Type -text -transform=snake -output=./type_enum.go
type Type int

const (
	Auto Type = iota
	Git  Type = iota
	Filesystem
)

type File struct {
	Path    string
	RelPath string
	Info    fs.FileInfo
}

func (f File) HasChanged() (bool, fs.FileInfo, error) {
	// get the file's current state
	current, err := os.Stat(f.Path)
	if err != nil {
		return false, nil, fmt.Errorf("failed to stat %s: %w", f.Path, err)
	}

	// check the size first
	if f.Info.Size() != current.Size() {
		return true, current, nil
	}

	// POSIX specifies EPOCH time for Mod time, but some filesystems give more precision.
	// Some formatters mess with the mod time (e.g. dos2unix) but not to the same precision,
	// triggering false positives.
	// We truncate everything below a second.
	if f.Info.ModTime().Truncate(time.Second) != current.ModTime().Truncate(time.Second) {
		return true, current, nil
	}

	return false, nil, nil
}

func (f File) String() string {
	return f.Path
}

type WalkFunc func(file *File, err error) error

type Walker interface {
	Root() string
	Walk(ctx context.Context, fn WalkFunc) error
}

func New(walkerType Type, root string, pathsCh chan string) (Walker, error) {
	switch walkerType {
	case Git:
		return NewGit(root, pathsCh)
	case Auto:
		return Detect(root, pathsCh)
	case Filesystem:
		return NewFilesystem(root, pathsCh)
	default:
		return nil, fmt.Errorf("unknown walker type: %v", walkerType)
	}
}

func Detect(root string, pathsCh chan string) (Walker, error) {
	// for now, we keep it simple and try git first, filesystem second
	w, err := NewGit(root, pathsCh)
	if err == nil {
		return w, err
	}
	return NewFilesystem(root, pathsCh)
}

func FindUp(searchDir string, fileNames ...string) (path string, dir string, err error) {
	for _, dir := range eachDir(searchDir) {
		for _, f := range fileNames {
			path := filepath.Join(dir, f)
			if fileExists(path) {
				return path, dir, nil
			}
		}
	}
	return "", "", fmt.Errorf("could not find %s in %s", fileNames, searchDir)
}

func eachDir(path string) (paths []string) {
	path, err := filepath.Abs(path)
	if err != nil {
		return
	}

	paths = []string{path}

	if path == "/" {
		return
	}

	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == os.PathSeparator {
			path = path[:i]
			if path == "" {
				path = "/"
			}
			paths = append(paths, path)
		}
	}

	return
}

func fileExists(path string) bool {
	// Some broken filesystems like SSHFS return file information on stat() but
	// then cannot open the file. So we use os.Open.
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	// Next, check that the file is a regular file.
	fi, err := f.Stat()
	if err != nil {
		return false
	}

	return fi.Mode().IsRegular()
}
