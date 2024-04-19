package format

import "context"

type Pipeline struct {
	sequence []*Formatter
}

func (p *Pipeline) Add(f *Formatter) {
	p.sequence = append(p.sequence, f)
}

func (p *Pipeline) Wants(path string) bool {
	var match bool
	for _, f := range p.sequence {
		match = f.Wants(path)
		if match {
			break
		}
	}
	return match
}

func (p *Pipeline) Apply(ctx context.Context, paths []string) error {
	for _, f := range p.sequence {
		if err := f.Apply(ctx, paths); err != nil {
			return err
		}
	}
	return nil
}
