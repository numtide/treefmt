package matcher

import (
	"github.com/numtide/treefmt/v2/walk"
)

type CompositeMatcher struct {
	matchers []Matcher
}

func NewCompositeMatcher(matchers []Matcher) *CompositeMatcher {
	return &CompositeMatcher{matchers: matchers}
}

func (cm *CompositeMatcher) Wants(file *walk.File) (Result, error) {
	// Default to "don't care."
	final := Indifferent

	for _, matcher := range cm.matchers {
		result, err := Wants(matcher, file)
		if err != nil {
			return Error, err
		}

		switch result {
		case Wanted:
			// Flip: at least one matcher wants this file.
			final = Wanted
		case Unwanted:
			// Short-circuit; we've rejected this file.
			return Unwanted, nil
		case Indifferent:
		case Error:
		default:
		}
	}

	return final, nil
}
