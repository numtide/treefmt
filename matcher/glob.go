package matcher

import (
	"fmt"

	"github.com/gobwas/glob"
	"github.com/numtide/treefmt/v2/walk"
)

func IncludeGlobs(patterns []string) (MatchFn, error) {
	if len(patterns) == 0 {
		return noOp, nil
	}

	globs := make([]glob.Glob, len(patterns))

	for i, pattern := range patterns {
		g, err := glob.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile glob pattern '%v': %w", pattern, err)
		}

		globs[i] = g
	}

	return func(file *walk.File) (Result, error) {
		for _, g := range globs {
			if g.Match(file.RelPath) {
				return Wanted, nil
			}
		}

		return Indifferent, nil
	}, nil
}

func ExcludeGlobs(patterns []string) (MatchFn, error) {
	includeFn, err := IncludeGlobs(patterns)

	return invert(includeFn), err
}
