package stats

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

//go:generate enumer -type=Type -text -transform=snake -output=./stats_type.go
type Type int

const (
	Traversed Type = iota
	Matched
	Formatted
	Changed
)

type Stats struct {
	start    time.Time
	counters map[Type]*atomic.Int32
}

func (s *Stats) Add(t Type, delta int32) int32 {
	return s.counters[t].Add(delta)
}

func (s *Stats) Value(t Type) int32 {
	return s.counters[t].Load()
}

func (s *Stats) Elapsed() time.Duration {
	return time.Since(s.start)
}

func (s *Stats) Print() {
	components := []string{
		"traversed %d files",
		"emitted %d files for processing",
		"formatted %d files (%d changed) in %v",
		"",
	}

	fmt.Printf(
		strings.Join(components, "\n"),
		s.Value(Traversed),
		s.Value(Matched),
		s.Value(Formatted),
		s.Value(Changed),
		s.Elapsed().Round(time.Millisecond),
	)
}

func New() Stats {
	// init counters
	counters := make(map[Type]*atomic.Int32)
	counters[Traversed] = &atomic.Int32{}
	counters[Matched] = &atomic.Int32{}
	counters[Formatted] = &atomic.Int32{}
	counters[Changed] = &atomic.Int32{}

	return Stats{
		start:    time.Now(),
		counters: counters,
	}
}
