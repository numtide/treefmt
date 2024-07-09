package format

import (
	"cmp"
	"slices"

	"git.numtide.com/numtide/treefmt/walker"
)

type Task struct {
	File       *walker.File
	Formatters []*Formatter
	BatchKey   string
}

func NewTask(file *walker.File, formatters []*Formatter) Task {
	// sort by priority in ascending order
	slices.SortFunc(formatters, func(a, b *Formatter) int {
		priorityA := a.Priority()
		priorityB := b.Priority()

		result := priorityA - priorityB
		if result == 0 {
			// formatters with the same priority are sorted lexicographically to ensure a deterministic outcome
			result = cmp.Compare(a.Name(), b.Name())
		}
		return result
	})

	// construct a batch key which represents the unique sequence of formatters to be applied to file
	var key string
	for _, f := range formatters {
		key += f.name + ":"
	}
	key = key[:len(key)-1]

	return Task{
		File:       file,
		Formatters: formatters,
		BatchKey:   key,
	}
}
