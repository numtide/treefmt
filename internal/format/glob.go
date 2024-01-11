package format

import (
	"fmt"

	"github.com/gobwas/glob"
)

// CompileGlobs prepares the globs, where the patterns are all right-matching.
func CompileGlobs(patterns []string) ([]glob.Glob, error) {
	globs := make([]glob.Glob, len(patterns))

	for i, pattern := range patterns {
		g, err := glob.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to compile include pattern '%v'", err, pattern)
		}
		globs[i] = g
	}

	return globs, nil
}

func PathMatches(path string, globs []glob.Glob) bool {
	for idx := range globs {
		if globs[idx].Match(path) {
			return true
		}
	}

	return false
}
