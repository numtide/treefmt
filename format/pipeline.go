package format

import (
	"context"
	"slices"

	"git.numtide.com/numtide/treefmt/walk"
)

type Pipeline struct {
	sequence []*Formatter
}

func (p *Pipeline) Add(f *Formatter) {
	p.sequence = append(p.sequence, f)
	// sort by priority in ascending order
	slices.SortFunc(p.sequence, func(a, b *Formatter) int {
		return a.config.Priority - b.config.Priority
	})
}

func (p *Pipeline) Wants(path *walk.File) bool {
	var match bool
	for _, f := range p.sequence {
		match = f.Wants(path)
		if match {
			break
		}
	}
	return match
}

func (p *Pipeline) Apply(ctx context.Context, paths []*walk.File) error {
	for _, f := range p.sequence {
		if err := f.Apply(ctx, paths, len(p.sequence) > 1); err != nil {
			return err
		}
	}
	return nil
}
