package stats

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

type Type int

const (
	Traversed Type = iota
	Emitted
	Matched
	Formatted
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
		s.Value(Emitted),
		s.Value(Matched),
		s.Value(Formatted),
		s.Elapsed().Round(time.Millisecond),
	)
}

func New() *Stats {
	// record start time
	start := time.Now()

	// init counters
	counters := make(map[Type]*atomic.Int32)
	counters[Traversed] = &atomic.Int32{}
	counters[Emitted] = &atomic.Int32{}
	counters[Matched] = &atomic.Int32{}
	counters[Formatted] = &atomic.Int32{}

	return &Stats{
		start:    start,
		counters: counters,
	}
}
