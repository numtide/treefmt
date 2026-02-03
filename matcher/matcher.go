package matcher

import (
	"github.com/numtide/treefmt/v2/walk"
)

type Result int

const (
	// File explicitly selected.
	Wanted Result = iota
	// File explicitly rejected.
	Unwanted
	// File neither selected nor rejected.
	Indifferent
	// Something went wrong.
	Error
)

type MatchFn = func(file *walk.File) (Result, error)

func noOp(_ *walk.File) (Result, error) {
	return Indifferent, nil
}

func invert(match MatchFn) MatchFn {
	return func(file *walk.File) (Result, error) {
		result, err := match(file)

		switch result {
		case Wanted:
			result = Unwanted
		case Unwanted:
			result = Wanted
		case Indifferent:
		case Error:
		}

		return result, err
	}
}

// Combine combines multiple matchers into a single matcher.
// The order of the matchers is important, which is why have explicit parameters for includes and excludes.
func Combine(includes []MatchFn, excludes []MatchFn) MatchFn {
	// Combine the matchers, ensuring exclusions are applied first.
	// This ensures that a file is rejected if it matches any of the excludes, even if it matches an include.
	matchers := make([]MatchFn, 0, len(excludes)+len(includes))
	matchers = append(matchers, excludes...)
	matchers = append(matchers, includes...)

	return func(file *walk.File) (Result, error) {
		var (
			err error
			// Default to "don't care."
			result = Indifferent
		)

		for _, matchFn := range matchers {
			result, err = matchFn(file)
			if err != nil {
				return Error, err
			}

			switch result {
			case Wanted, Unwanted:
				return result, nil

			case Indifferent:
			case Error:
			default:
			}
		}

		return result, nil
	}
}
