package matcher_test

import (
	"testing"

	"github.com/numtide/treefmt/v2/matcher"
	"github.com/numtide/treefmt/v2/matcher/internal/testutil"
	"github.com/numtide/treefmt/v2/walk"
	"github.com/stretchr/testify/require"
)

func testGlobMatcher(t *testing.T, as *require.Assertions, create func([]string) (matcher.Matcher, error)) {
	t.Helper()

	f := func(path string) *walk.File {
		return &walk.File{
			Path:    path,
			RelPath: path,
		}
	}

	var (
		m      matcher.Matcher
		err    error
		target matcher.Result
	)

	// Empty list; `Wants` should always return `true`.
	m, err = create([]string{})
	as.NoError(err)
	testutil.MatcherTestEmpty(t, as, m)

	if m.Invert() {
		target = matcher.Unwanted
	} else {
		target = matcher.Wanted
	}

	// File extension
	m, err = create([]string{"*.txt"})
	as.NoError(err)
	testutil.MatcherTestResults(t, as, m, map[matcher.Result][]*walk.File{
		target:              {f("test/foo/bar.txt")},
		matcher.Indifferent: {f("test/foo/bar.txtz"), f("test/foo/bar.flob")},
	})

	// Prefix matching
	m, err = create([]string{"test/*"})
	as.NoError(err)
	testutil.MatcherTestResults(t, as, m, map[matcher.Result][]*walk.File{
		target:              {f("test/bar.txt"), f("test/foo/bar.txt")},
		matcher.Indifferent: {f("/test/foo/bar.txt")},
	})

	// Exact matches
	// File extension
	m, err = create([]string{"LICENSE"})
	as.NoError(err)
	testutil.MatcherTestResults(t, as, m, map[matcher.Result][]*walk.File{
		target:              {f("LICENSE")},
		matcher.Indifferent: {f("test/LICENSE"), f("LICENSE.txt")},
	})
}

func TestIncludes(t *testing.T) {
	r := require.New(t)

	testGlobMatcher(t, r, matcher.NewGlobInclusionMatcher)
}

func TestExcludes(t *testing.T) {
	r := require.New(t)

	testGlobMatcher(t, r, matcher.NewGlobExclusionMatcher)
}
