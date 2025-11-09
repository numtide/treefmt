package matcher

import (
	"fmt"

	"github.com/gobwas/glob"
	"github.com/numtide/treefmt/v2/walk"
)

type globMatcher struct {
	globs    []glob.Glob
	patterns []string
}

func newGlobMatcher(patterns []string) (*globMatcher, error) {
	globs := make([]glob.Glob, len(patterns))

	for i, pattern := range patterns {
		g, err := glob.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile include pattern '%v': %w", pattern, err)
		}

		globs[i] = g
	}

	return &globMatcher{globs: globs, patterns: patterns}, nil
}

func (gm *globMatcher) Ignore() bool {
	return len(gm.globs) < 1
}

func (gm *globMatcher) Matches(file *walk.File) (bool, error) {
	for _, g := range gm.globs {
		if g.Match(file.RelPath) {
			return true, nil
		}
	}

	return false, nil
}

type GlobInclusionMatcher struct {
	*inclusionMatcher
	globMatcher
}

//nolint:ireturn
func NewGlobInclusionMatcher(patterns []string) (Matcher, error) {
	gm, err := newGlobMatcher(patterns)
	if err != nil {
		return nil, err
	}

	return &GlobInclusionMatcher{globMatcher: *gm}, nil
}

type GlobExclusionMatcher struct {
	*exclusionMatcher
	globMatcher
}

//nolint:ireturn
func NewGlobExclusionMatcher(patterns []string) (Matcher, error) {
	gm, err := newGlobMatcher(patterns)
	if err != nil {
		return nil, err
	}

	return &GlobExclusionMatcher{globMatcher: *gm}, nil
}
