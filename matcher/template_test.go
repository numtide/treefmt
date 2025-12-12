package matcher_test

import (
	"testing"

	"github.com/numtide/treefmt/v2/matcher"
	"github.com/numtide/treefmt/v2/matcher/internal/testutil"
	"github.com/numtide/treefmt/v2/walk"
	"github.com/stretchr/testify/require"
)

func TestIncludeTemplates(t *testing.T) {
	as := require.New(t)

	f := testutil.MatcherTestSetup(t, as)

	var (
		err     error
		matchFn matcher.MatchFn
	)

	// Empty list; `Wants` should always return `true`.
	matchFn, err = matcher.IncludeTemplates([]string{})
	as.NoError(err)

	result, err := matchFn(&walk.File{RelPath: "test/foo/bar.txt"})
	as.NoError(err)
	as.Equal(matcher.Indifferent, result)

	matchFn, err = matcher.IncludeTemplates([]string{
		"{{ rematch `[[:space:]]perl(?:[[:space:]]|$)` .Shebang }}",
		"{{ eq `ruby` .InterpreterName }}",
	})
	as.NoError(err)

	testutil.MatcherTestResults(t, as, matchFn, map[matcher.Result][]*walk.File{
		matcher.Wanted:      {f("perl-script"), f("ruby-script")},
		matcher.Indifferent: {f("python-script"), f("shell-script")},
	})
}

func TestReject(t *testing.T) {
	as := require.New(t)

	f := testutil.MatcherTestSetup(t, as)

	var (
		matchFn matcher.MatchFn
		err     error
	)

	// Empty list; `Wants` should always return `true`.
	matchFn, err = matcher.ExcludeTemplates([]string{})
	as.NoError(err)

	result, err := matchFn(&walk.File{RelPath: "test/foo/bar.txt"})
	as.NoError(err)
	as.Equal(matcher.Indifferent, result)

	matchFn, err = matcher.ExcludeTemplates([]string{
		"{{ fnmatch `python*` .Interpreter }}",
		"{{ rematch `/bin/(?:(?:b|d)?a)?sh` .Interpreter }}",
	})
	as.NoError(err)
	testutil.MatcherTestResults(t, as, matchFn, map[matcher.Result][]*walk.File{
		matcher.Indifferent: {f("perl-script"), f("ruby-script")},
		matcher.Unwanted:    {f("python-script"), f("shell-script")},
	})
}
