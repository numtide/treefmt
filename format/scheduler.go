package format

import (
	"bytes"
	"cmp"
	"context"
	"crypto/md5" //nolint:gosec
	"fmt"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/log"
	"github.com/numtide/treefmt/stats"
	"github.com/numtide/treefmt/walk"
	"golang.org/x/sync/errgroup"
)

type (
	// batch represents a collection of File pointers to be processed together.
	batch []*walk.File

	// batchKey represents the unique sequence of formatters to be applied to a batch of files.
	// For example, "deadnix:statix:nixpkgs-fmt" indicates that deadnix should be applied first, statix second and
	// nixpkgs-fmt third.
	// Files are batched based on their formatting sequence, as determined by the priority and includes/excludes in the
	// formatter configuration.
	batchKey string

	// signature is a sha256 hash of a sequence of formatters.
	signature []byte
)

// sequence returns the list of formatters, by name, to be applied to a batch of files.
func (b batchKey) sequence() []string {
	return strings.Split(string(b), batchKeySeparator)
}

// newBatchKey takes a list of Formatters and returns a batchKey string composed of their names joined by ":".
func newBatchKey(formatters []*Formatter) batchKey {
	components := make([]string, 0, len(formatters))
	for _, f := range formatters {
		components = append(components, f.Name())
	}

	return batchKey(strings.Join(components, batchKeySeparator))
}

type scheduler struct {
	batchSize   int
	changeLevel log.Level
	formatters  map[string]*Formatter

	eg    *errgroup.Group
	stats *stats.Stats

	batches    map[batchKey]batch
	signatures map[batchKey]signature

	// formatError indicates if at least one formatting error occurred
	formatError *atomic.Bool
}

func (s *scheduler) formattersSignature(key batchKey, formatters []*Formatter) ([]byte, error) {
	sig, ok := s.signatures[key]
	if ok {
		// return pre-computed signature
		return sig, nil
	}

	// generate a signature by hashing each formatter in order
	h := md5.New() //nolint:gosec
	for _, f := range formatters {
		if err := f.Hash(h); err != nil {
			return nil, fmt.Errorf("failed to hash formatter %s: %w", f.Name(), err)
		}
	}

	sig = h.Sum(nil)

	// store the signature so we don't have to re-compute for each file
	s.signatures[key] = sig

	return sig, nil
}

func (s *scheduler) submit(
	ctx context.Context,
	file *walk.File,
	matches []*Formatter,
) (accepted bool, err error) {
	slices.SortFunc(matches, formatterSortFunc)

	// construct a batch key based on the sequence of formatters
	key := newBatchKey(matches)

	// get format signature
	formattersSig, err := s.formattersSignature(key, matches)
	if err != nil {
		return false, fmt.Errorf("failed to get formatter's signature: %w", err)
	}

	// calculate the overall signature
	signature, err := file.FormatSignature(formattersSig)
	if err != nil {
		return false, fmt.Errorf("failed to calculate file signature: %w", err)
	}

	// compare signature with last cache entry
	if bytes.Equal(signature, file.CachedFormatSignature) {
		// If the signature is the same as the last cache entry, there is nothing to do.
		// We know from the hash signature that we have already applied this sequence of formatters (and their config) to
		// this file.
		// When we applied the formatters, the file had the same mod time and file size.
		return false, nil
	}

	// append the formatters sig to the file
	// it will be necessary later to calculate a new format signature
	file.FormattersSignature = formattersSig

	// append to the batch
	s.batches[key] = append(s.batches[key], file)

	// schedule the batch for processing if it's full
	if len(s.batches[key]) == s.batchSize {
		s.schedule(ctx, key, s.batches[key])
		// reset the batch
		s.batches[key] = make([]*walk.File, 0, s.batchSize)
	}

	return true, nil
}

// schedule begins processing a batch in the background.
func (s *scheduler) schedule(ctx context.Context, key batchKey, batch []*walk.File) {
	s.eg.Go(func() error {
		var formatErrors []error

		// apply the formatters in sequence
		for _, name := range key.sequence() {
			formatter := s.formatters[name]

			if err := formatter.Apply(ctx, batch); err != nil {
				formatErrors = append(formatErrors, err)
			}
		}

		// record if a format error occurred
		hasErrors := len(formatErrors) > 0

		// update overall error tracking
		s.formatError.Store(hasErrors)

		if !hasErrors {
			// record that the file was formatted
			s.stats.Add(stats.Formatted, len(batch))
		}

		// Create a release context.
		// We set no-cache based on whether any formatting errors occurred in this batch.
		// This is to communicate with any caching layer, if used when reading files for this batch, that it should not
		// update the state of any file in this batch, as we want to re-process them in later invocations.
		releaseCtx := walk.SetNoCache(ctx, hasErrors)

		// post-processing
		for _, file := range batch {
			// check if the file has changed
			changed, newInfo, err := file.Stat()
			if err != nil {
				return fmt.Errorf("failed to stat file: %w", err)
			}

			if changed {
				// record the change
				s.stats.Add(stats.Changed, 1)

				// log the change (useful for diagnosing issues)
				log.Log(
					s.changeLevel, "file has changed",
					"path", file.RelPath,
					"prev_size", file.Info.Size(),
					"prev_mod_time", file.Info.ModTime().Truncate(time.Second),
					"current_size", newInfo.Size(),
					"current_mod_time", newInfo.ModTime().Truncate(time.Second),
				)

				// record the new file info
				file.FormattedInfo = newInfo
			}

			// release the file as there is no further processing to be done on it
			if err := file.Release(releaseCtx); err != nil {
				return fmt.Errorf("failed to release file: %w", err)
			}
		}

		return nil
	})
}

func (s *scheduler) close(ctx context.Context) error {
	// schedule any partial batches that remain
	for key, batch := range s.batches {
		if len(batch) > 0 {
			s.schedule(ctx, key, batch)
		}
	}

	// wait for processing to complete
	if err := s.eg.Wait(); err != nil {
		return fmt.Errorf("failed to wait for formatters: %w", err)
	} else if s.formatError.Load() {
		return ErrFormattingFailures
	}

	return nil
}

// formatterSortFunc sorts formatters by their priority in ascending order; ties are resolved by lexicographic order of
// names.
func formatterSortFunc(a, b *Formatter) int {
	// sort by priority in ascending order
	priorityA := a.Priority()
	priorityB := b.Priority()

	result := priorityA - priorityB
	if result == 0 {
		// formatters with the same priority are sorted lexicographically to ensure a deterministic outcome
		result = cmp.Compare(a.Name(), b.Name())
	}

	return result
}

func newScheduler(
	statz *stats.Stats,
	batchSize int,
	changeLevel log.Level,
	formatters map[string]*Formatter,
) *scheduler {
	eg := &errgroup.Group{}
	// we use a simple heuristic to avoid too much contention by limiting the concurrency to runtime.NumCPU()
	eg.SetLimit(runtime.NumCPU())

	return &scheduler{
		batchSize:   batchSize,
		changeLevel: changeLevel,
		formatters:  formatters,

		eg:    eg,
		stats: statz,

		batches:     make(map[batchKey]batch),
		signatures:  make(map[batchKey]signature),
		formatError: &atomic.Bool{},
	}
}
