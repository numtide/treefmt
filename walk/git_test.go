package walk

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestFileTree(t *testing.T) {

	as := require.New(t)

	node := &fileTree{name: ""}
	node.addPath("foo/bar/baz")
	node.addPath("fizz/buzz")
	node.addPath("hello/world")
	node.addPath("foo/bar/fizz")
	node.addPath("foo/fizz/baz")

	as.True(node.hasPath("foo"))
	as.True(node.hasPath("foo/bar"))
	as.True(node.hasPath("foo/bar/baz"))
	as.True(node.hasPath("fizz"))
	as.True(node.hasPath("fizz/buzz"))
	as.True(node.hasPath("hello"))
	as.True(node.hasPath("hello/world"))
	as.True(node.hasPath("foo/bar/fizz"))
	as.True(node.hasPath("foo/fizz/baz"))

	as.False(node.hasPath("fo"))
	as.False(node.hasPath("world"))
}
