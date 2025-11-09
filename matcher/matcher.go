package matcher

import (
	"fmt"

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

func Wants(m Matcher, file *walk.File) (Result, error) {
	if m.Ignore() {
		return Indifferent, nil
	}

	match, err := m.Matches(file)
	if err != nil {
		return Error, fmt.Errorf("error applying matcher to %s: %w", file.RelPath, err)
	}

	if match {
		if m.Invert() {
			return Unwanted, nil
		}

		return Wanted, nil
	}

	return Indifferent, nil
}

type Matcher interface {
	Matches(file *walk.File) (bool, error)
	Ignore() bool
	Invert() bool
}

type inclusionMatcher struct{}

func (*inclusionMatcher) Invert() bool {
	return false
}

type exclusionMatcher struct{}

func (*exclusionMatcher) Invert() bool {
	return true
}
