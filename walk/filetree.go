package walk

import (
	"bufio"
	"io"
	"path/filepath"
	"strings"
)

// filetree represents a hierarchical file structure with directories and files.
type filetree struct {
	name    string
	entries map[string]*filetree
}

// add inserts a file path into the filetree structure, creating necessary parent directories if they do not exist.
func (n *filetree) add(path []string) {
	if len(path) == 0 {
		return
	} else if n.entries == nil {
		n.entries = make(map[string]*filetree)
	}

	name := path[0]
	child, ok := n.entries[name]
	if !ok {
		child = &filetree{name: name}
		n.entries[name] = child
	}
	child.add(path[1:])
}

// addPath splits the given path by the filepath separator and inserts it into the filetree structure.
func (n *filetree) addPath(path string) {
	n.add(strings.Split(path, string(filepath.Separator)))
}

// has returns true if the specified path exists in the filetree, false otherwise.
func (n *filetree) has(path []string) bool {
	if len(path) == 0 {
		return true
	} else if len(n.entries) == 0 {
		return false
	}
	child, ok := n.entries[path[0]]
	if !ok {
		return false
	}
	return child.has(path[1:])
}

// hasPath splits the given path by the filepath separator and checks if it exists in the filetree.
func (n *filetree) hasPath(path string) bool {
	if path == "." {
		return true
	}
	return n.has(strings.Split(path, string(filepath.Separator)))
}

func (n *filetree) getPath(path string) (*filetree, bool) {
	if path == "." {
		return n, true
	}
	return n.get(strings.Split(path, string(filepath.Separator)))
}

func (n *filetree) get(path []string) (*filetree, bool) {
	if len(path) == 0 {
		return n, true
	} else if len(n.entries) == 0 {
		return nil, false
	}

	child, ok := n.entries[path[0]]
	if !ok {
		return nil, false
	}

	return child.get(path[1:])
}

func (n *filetree) walk(prefix string, fn func(path string) error) error {
	path := filepath.Join(prefix, n.name)

	if len(n.entries) == 0 {
		if err := fn(path); err != nil {
			return err
		}
	}

	for _, child := range n.entries {
		if err := child.walk(path, fn); err != nil {
			return err
		}
	}

	return nil
}

func (n *filetree) fromGit(r io.Reader) {
	scan := bufio.NewScanner(r)
	for scan.Scan() {
		text := scan.Text()

		// we're only interested in staged changes
		switch text[0] {
		case 'A', 'M', 'R', 'C':
			n.addPath(text[3:])
		default:
			// do nothing
		}
	}
}
