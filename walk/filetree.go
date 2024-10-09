package walk

import (
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/index"
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
	return n.has(strings.Split(path, string(filepath.Separator)))
}

// readIndex traverses the index entries and adds each file path to the filetree structure.
func (n *filetree) readIndex(idx *index.Index) {
	for _, entry := range idx.Entries {
		n.addPath(entry.Name)
	}
}
