package walk

import "github.com/numtide/treefmt/v2/stats"

type CustomReader = PathStreamReader

func NewCustomReader(
	root string,
	pathFilters []string,
	statz *stats.Stats,
	cfg CustomConfig,
) (*CustomReader, error) {
	return NewPathStreamReader(root, statz, PathStreamConfig{
		Name:        "custom walker " + cfg.Name,
		Command:     cfg.Command,
		Options:     cfg.Options,
		PathFilters: pathFilters,
	})
}
