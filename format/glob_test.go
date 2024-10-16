//nolint:testpackage
package format

import (
	"testing"

	"github.com/gobwas/glob"
	"github.com/stretchr/testify/require"
)

func TestGlobs(t *testing.T) {
	r := require.New(t)

	var (
		globs []glob.Glob
		err   error
	)

	// File extension
	globs, err = compileGlobs([]string{"*.txt"})
	r.NoError(err)
	r.True(pathMatches("test/foo/bar.txt", globs))
	r.False(pathMatches("test/foo/bar.txtz", globs))
	r.False(pathMatches("test/foo/bar.flob", globs))

	// Prefix matching
	globs, err = compileGlobs([]string{"test/*"})
	r.NoError(err)
	r.True(pathMatches("test/bar.txt", globs))
	r.True(pathMatches("test/foo/bar.txt", globs))
	r.False(pathMatches("/test/foo/bar.txt", globs))

	// Exact matches
	// File extension
	globs, err = compileGlobs([]string{"LICENSE"})
	r.NoError(err)
	r.True(pathMatches("LICENSE", globs))
	r.False(pathMatches("test/LICENSE", globs))
	r.False(pathMatches("LICENSE.txt", globs))
}
