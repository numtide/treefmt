package testutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/numtide/treefmt/v2/matcher"
	"github.com/numtide/treefmt/v2/walk"
	"github.com/stretchr/testify/require"
)

func MatcherTestSetup(t *testing.T, as *require.Assertions) func(string) *walk.File {
	t.Helper()

	// r := require.New(t)

	cwd, err := os.Getwd()
	as.NoError(err)

	testfile := func(name string) *walk.File {
		relpath := filepath.Join("testdata", name)
		// fileinfo, err := os.Stat(relpath)
		// as.NoError(err)
		// as.True(fileinfo.Mode().IsRegular(), "path %s is not a regular file", relpath)

		return &walk.File{
			RelPath: relpath,
			Path:    filepath.Join(cwd, relpath),
		}
	}

	return testfile
}

func MatcherTestEmpty(t *testing.T, as *require.Assertions, m matcher.Matcher) {
	t.Helper()

	as.True(m.Ignore(), "matcher is not configured to be ignored")

	result, err := matcher.Wants(m, &walk.File{RelPath: "ENOENT"})
	as.NoError(err)
	as.Equal(matcher.Indifferent, result)
}

func MatcherTestResults(
	t *testing.T,
	as *require.Assertions,
	m matcher.Matcher,
	results map[matcher.Result][]*walk.File,
) {
	t.Helper()

	as.False(m.Ignore(), "matcher is configured to be ignored")

	for expected, paths := range results {
		for _, path := range paths {
			actual, err := matcher.Wants(m, path)
			as.NoError(err)
			as.Equal(expected, actual, "expected %v for path %s; got %v", expected, path, actual)
		}
	}
}
