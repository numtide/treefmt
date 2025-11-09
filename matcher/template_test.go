package matcher_test

import (
	"testing"

	"github.com/numtide/treefmt/v2/matcher"
	"github.com/numtide/treefmt/v2/matcher/internal/testutil"
	"github.com/numtide/treefmt/v2/walk"
	"github.com/stretchr/testify/require"
)

func TestAccept(t *testing.T) {
	r := require.New(t)

	f := testutil.MatcherTestSetup(t, r)

	var (
		m   matcher.Matcher
		err error
	)

	// Empty list; `Wants` should always return `true`.
	m, err = matcher.NewTemplateInclusionMatcher([]string{})
	r.NoError(err)
	testutil.MatcherTestEmpty(t, r, m)

	m, err = matcher.NewTemplateInclusionMatcher([]string{
		"{{ rematch `[[:space:]]perl(?:[[:space:]]|$)` .Shebang }}",
		"{{ eq `ruby` .InterpreterName }}",
	})
	r.NoError(err)
	testutil.MatcherTestResults(t, r, m, map[matcher.Result][]*walk.File{
		matcher.Wanted:      {f("perl-script"), f("ruby-script")},
		matcher.Indifferent: {f("python-script"), f("shell-script")},
	})
}

func TestReject(t *testing.T) {
	r := require.New(t)

	f := testutil.MatcherTestSetup(t, r)

	var (
		m   matcher.Matcher
		err error
	)

	// Empty list; `Wants` should always return `true`.
	m, err = matcher.NewTemplateExclusionMatcher([]string{})
	r.NoError(err)
	testutil.MatcherTestEmpty(t, r, m)

	m, err = matcher.NewTemplateExclusionMatcher([]string{
		"{{ fnmatch `python*` .Interpreter }}",
		"{{ rematch `/bin/(?:(?:b|d)?a)?sh` .Interpreter }}",
	})
	r.NoError(err)
	testutil.MatcherTestResults(t, r, m, map[matcher.Result][]*walk.File{
		matcher.Indifferent: {f("perl-script"), f("ruby-script")},
		matcher.Unwanted:    {f("python-script"), f("shell-script")},
	})
}
