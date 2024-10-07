package format_test

import (
	"testing"

	"github.com/gobwas/glob"
	"github.com/numtide/treefmt/format"
	"github.com/stretchr/testify/require"
)

func TestGlobs(t *testing.T) {
	r := require.New(t)

	var (
		globs []glob.Glob
		err   error
	)

	// File extension
	globs, err = format.CompileGlobs([]string{"*.txt"})
	r.NoError(err)
	r.True(format.PathMatches("test/foo/bar.txt", globs))
	r.False(format.PathMatches("test/foo/bar.txtz", globs))
	r.False(format.PathMatches("test/foo/bar.flob", globs))

	// Prefix matching
	globs, err = format.CompileGlobs([]string{"test/*"})
	r.NoError(err)
	r.True(format.PathMatches("test/bar.txt", globs))
	r.True(format.PathMatches("test/foo/bar.txt", globs))
	r.False(format.PathMatches("/test/foo/bar.txt", globs))

	// Exact matches
	// File extension
	globs, err = format.CompileGlobs([]string{"LICENSE"})
	r.NoError(err)
	r.True(format.PathMatches("LICENSE", globs))
	r.False(format.PathMatches("test/LICENSE", globs))
	r.False(format.PathMatches("LICENSE.txt", globs))
}
