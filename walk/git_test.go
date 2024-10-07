package walk_test

import (
	"testing"

	"github.com/numtide/treefmt/walk"
	"github.com/stretchr/testify/require"
)

func TestFileTree(t *testing.T) {
	as := require.New(t)

	node := &walk.FileTree{Name: ""}
	node.AddPath("foo/bar/baz")
	node.AddPath("fizz/buzz")
	node.AddPath("hello/world")
	node.AddPath("foo/bar/fizz")
	node.AddPath("foo/fizz/baz")

	as.True(node.HasPath("foo"))
	as.True(node.HasPath("foo/bar"))
	as.True(node.HasPath("foo/bar/baz"))
	as.True(node.HasPath("fizz"))
	as.True(node.HasPath("fizz/buzz"))
	as.True(node.HasPath("hello"))
	as.True(node.HasPath("hello/world"))
	as.True(node.HasPath("foo/bar/fizz"))
	as.True(node.HasPath("foo/fizz/baz"))

	as.False(node.HasPath("fo"))
	as.False(node.HasPath("world"))
}
