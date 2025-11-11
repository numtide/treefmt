//nolint:testpackage
package format

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGlobs(t *testing.T) {
	r := require.New(t)

	var (
		matcher *globMatcher
		err     error
	)

	cache := NewMatcherCache()

	// File extension
	matcher, err = newGlobMatcher([]string{"*.txt"})
	r.NoError(err)
	r.True(matcher.MatchesPath("test/foo/bar.txt", cache))
	r.False(matcher.MatchesPath("test/foo/bar.txtz", cache))
	r.False(matcher.MatchesPath("test/foo/bar.flob", cache))

	// Prefix matching
	matcher, err = newGlobMatcher([]string{"test/*"})
	r.NoError(err)
	r.True(matcher.MatchesPath("test/bar.txt", cache))
	r.True(matcher.MatchesPath("test/foo/bar.txt", cache))
	r.False(matcher.MatchesPath("/test/foo/bar.txt", cache))

	// Exact matches
	// File extension
	matcher, err = newGlobMatcher([]string{"LICENSE"})
	r.NoError(err)
	r.True(matcher.MatchesPath("LICENSE", cache))
	r.False(matcher.MatchesPath("test/LICENSE", cache))
	r.False(matcher.MatchesPath("LICENSE.txt", cache))
}
