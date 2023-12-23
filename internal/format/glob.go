package format

import (
	"github.com/gobwas/glob"
)

func PathMatches(path string, globs []glob.Glob) bool {
	for idx := range globs {
		if globs[idx].Match(path) {
			return true
		}
	}

	return false
}
