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

var (
	counters map[Type]*atomic.Int32
	start    time.Time
)

func Init() {
	// record start time
	start = time.Now()

	// init counters
	counters = make(map[Type]*atomic.Int32)
	counters[Traversed] = &atomic.Int32{}
	counters[Emitted] = &atomic.Int32{}
	counters[Matched] = &atomic.Int32{}
	counters[Formatted] = &atomic.Int32{}
}

func Add(t Type, delta int32) int32 {
	return counters[t].Add(delta)
}

func Value(t Type) int32 {
	return counters[t].Load()
}

func Elapsed() time.Duration {
	return time.Since(start)
}

func Print() {
	components := []string{
		"traversed %d files",
		"emitted %d files for processing",
		"matched %d files to formatters",
		"formatted %d files in %v",
		"",
	}

	fmt.Printf(
		strings.Join(components, "\n"),
		Value(Traversed),
		Value(Emitted),
		Value(Matched),
		Value(Formatted),
		Elapsed().Round(time.Millisecond),
	)
}
