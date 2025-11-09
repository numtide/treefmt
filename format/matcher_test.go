//nolint:testpackage
package format

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

func mimetypeTestSetup(t *testing.T) (*require.Assertions, func(string) string) {
	t.Helper()

	r := require.New(t)

	cwd, err := os.Getwd()
	r.NoError(err)

	testdata := path.Join(cwd, "testdata")
	fileinfo, err := os.Stat(testdata)
	r.NoError(err)
	r.True(fileinfo.Mode().IsDir(), "path %s is not a directory", testdata)

	testfile := func(relpath string) string {
		fullpath := path.Join(testdata, relpath)
		fileinfo, err := os.Stat(fullpath)
		r.NoError(err)
		r.True(fileinfo.Mode().IsRegular(), "path %s is not a regular file", fullpath)

		return fullpath
	}

	return r, testfile
}

func TestAllowedMimetypes(t *testing.T) {
	r, testfile := mimetypeTestSetup(t)

	var (
		matcher *MimetypeInclusionMatcher
		err     error
	)

	cache := NewMatcherCache()

	// Empty list; `MatcherWants` should always return `true`.
	matcher, err = NewMimetypeInclusionMatcher([]string{})
	r.NoError(err)
	r.True(matcher.Ignore(), "matcher is not configured to be ignored")
	r.True(MatcherWants(matcher, "ENOENT", cache))

	matcher, err = NewMimetypeInclusionMatcher([]string{
		"text/x-perl",
		"text/x-ruby",
	})
	r.NoError(err)
	r.False(matcher.Ignore(), "matcher is configured to be ignored")
	r.True(MatcherWants(matcher, testfile("perl-script"), cache))
	r.True(MatcherWants(matcher, testfile("ruby-script"), cache))
	r.False(MatcherWants(matcher, testfile("python-script"), cache))
	r.False(MatcherWants(matcher, testfile("shell-script"), cache))
}

func TestDisallowedMimetypes(t *testing.T) {
	r, testfile := mimetypeTestSetup(t)

	var (
		matcher *MimetypeExclusionMatcher
		err     error
	)

	cache := NewMatcherCache()

	// Empty list; `MatcherWants` should always return `true`.
	matcher, err = NewMimetypeExclusionMatcher([]string{})
	r.NoError(err)
	r.True(matcher.Ignore(), "matcher is not configured to be ignored")
	r.True(MatcherWants(matcher, "ENOENT", cache))

	matcher, err = NewMimetypeExclusionMatcher([]string{
		"text/x-script.python",
		"text/x-python",
		"text/x-shellscript",
	})
	r.NoError(err)
	r.False(matcher.Ignore(), "matcher is configured to be ignored")
	r.True(MatcherWants(matcher, testfile("perl-script"), cache))
	r.True(MatcherWants(matcher, testfile("ruby-script"), cache))
	r.False(MatcherWants(matcher, testfile("python-script"), cache))
	r.False(MatcherWants(matcher, testfile("shell-script"), cache))
}
