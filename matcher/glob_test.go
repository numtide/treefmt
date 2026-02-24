package matcher_test

import (
	"testing"

	"github.com/numtide/treefmt/v2/matcher"
	"github.com/numtide/treefmt/v2/matcher/internal/testutil"
	"github.com/numtide/treefmt/v2/walk"
	"github.com/stretchr/testify/require"
)

func mockFile(t *testing.T, path string) *walk.File {
	t.Helper()

	return &walk.File{
		Path:    path,
		RelPath: path,
	}
}

func testGlobMatcher(t *testing.T, factory func(patterns []string) (matcher.MatchFn, error), expected matcher.Result) {
	t.Helper()

	as := require.New(t)

	// Empty list; `Wants` should always return `true`.
	matchFn, err := factory(nil)
	as.NoError(err)

	result, err := matchFn(&walk.File{RelPath: "test/foo/bar.txt"})
	as.NoError(err)
	as.Equal(matcher.Indifferent, result)

	// File extension
	matchFn, err = factory([]string{"*.txt"})
	as.NoError(err)

	testutil.MatcherTestResults(t, as, matchFn, map[matcher.Result][]*walk.File{
		expected:            {mockFile(t, "test/foo/bar.txt")},
		matcher.Indifferent: {mockFile(t, "test/foo/bar.txtz"), mockFile(t, "test/foo/bar.flob")},
	})

	// Prefix matching
	matchFn, err = factory([]string{"test/*"})
	as.NoError(err)
	testutil.MatcherTestResults(t, as, matchFn, map[matcher.Result][]*walk.File{
		expected:            {mockFile(t, "test/bar.txt"), mockFile(t, "test/foo/bar.txt")},
		matcher.Indifferent: {mockFile(t, "/test/foo/bar.txt")},
	})

	// Exact matches
	// File extension
	matchFn, err = factory([]string{"LICENSE"})
	as.NoError(err)
	testutil.MatcherTestResults(t, as, matchFn, map[matcher.Result][]*walk.File{
		expected:            {mockFile(t, "LICENSE")},
		matcher.Indifferent: {mockFile(t, "test/LICENSE"), mockFile(t, "LICENSE.txt")},
	})
}

func TestIncludeGlobs(t *testing.T) {
	testGlobMatcher(t, matcher.IncludeGlobs, matcher.Wanted)
}

func TestExcludeGlobs(t *testing.T) {
	testGlobMatcher(t, matcher.ExcludeGlobs, matcher.Unwanted)
}
