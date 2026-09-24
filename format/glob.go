package format

import (
	"fmt"

	"github.com/gobwas/glob"
)

// compileGlobs prepares the globs, where the patterns are all right-matching.
func compileGlobs(patterns []string) ([]glob.Pattern, error) {
	globs := make([]glob.Pattern, len(patterns))

	for i, pattern := range patterns {
		g, err := glob.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile include pattern '%v': %w", pattern, err)
		}

		globs[i] = *g
	}

	return globs, nil
}

func pathMatches(path string, globs []glob.Pattern) bool {
	for idx := range globs {
		if globs[idx].Match(path) {
			return true
		}
	}

	return false
}
