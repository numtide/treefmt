package walk

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"

	"github.com/charmbracelet/log"
	"github.com/numtide/treefmt/walk/cache"
	bolt "go.etcd.io/bbolt"
	"golang.org/x/sync/errgroup"
)

// CachedReader reads files from a delegate Reader, appending a cache Entry on read (if on exists) and updating the
// cache after the file has been processed.
type CachedReader struct {
	db        *bolt.DB
	log       *log.Logger
	batchSize int

	// delegate is a Reader instance that performs the actual reading operations for the CachedReader.
	delegate Reader

	eg *errgroup.Group

	// updateCh contains files which have been released after processing and should be updated in the cache.
	updateCh chan *File
}

// process updates cached file entries by batching file updates and flushing them to the database periodically.
func (c *CachedReader) process() error {
	var batch []*File

	flush := func() error {
		// check for an empty batch
		if len(batch) == 0 {
			return nil
		}

		return c.db.Update(func(tx *bolt.Tx) error {
			// get the paths bucket
			bucket, err := cache.BucketPaths(tx)
			if err != nil {
				return fmt.Errorf("failed to get bucket: %w", err)
			}

			// for each file in the batch, add a new cache entry with update size and mod time.
			for _, file := range batch {
				entry := &cache.Entry{
					Size:     file.Info.Size(),
					Modified: file.Info.ModTime(),
				}
				if err = bucket.Put(file.RelPath, entry); err != nil {
					return fmt.Errorf("failed to put entry for path %s: %w", file.RelPath, err)
				}
			}
			return nil
		})
	}

	for file := range c.updateCh {
		batch = append(batch, file)
		if len(batch) == c.batchSize {
			if err := flush(); err != nil {
				return err
			}
			batch = batch[:0]
		}
	}

	// flush final partial batch
	return flush()
}

func (c *CachedReader) Read(ctx context.Context, files []*File) (n int, err error) {
	err = c.db.View(func(tx *bolt.Tx) error {
		// get paths bucket
		bucket, err := cache.BucketPaths(tx)
		if err != nil {
			return fmt.Errorf("failed to get bucket: %w", err)
		}

		// perform a read on the underlying reader
		n, err = c.delegate.Read(ctx, files)
		c.log.Debugf("read %d files from delegate", n)

		for i := 0; i < n; i++ {
			file := files[i]

			// lookup cache entry and append to the file
			file.Cache, err = bucket.Get(file.RelPath)
			if err != nil {
				return err
			}

			// set a release function which inserts this file into the update channel
			file.AddReleaseFunc(func(formatErr error) error {
				// in the event of a formatting error, we do not want to update this file in the cache
				// this ensures later invocations will try and re-format this file
				if formatErr == nil {
					c.updateCh <- file
				}

				return nil
			})
		}

		if errors.Is(err, io.EOF) {
			return err
		} else if err != nil {
			return fmt.Errorf("failed to read files from delegate: %w", err)
		}

		return nil
	})

	return n, err
}

// Close waits for any processing to complete.
func (c *CachedReader) Close() error {
	// close the release channel
	close(c.updateCh)

	// wait for any pending releases to be processed
	return c.eg.Wait()
}

// NewCachedReader creates a cache Reader instance, backed by a bolt DB and delegating reads to delegate.
func NewCachedReader(db *bolt.DB, batchSize int, delegate Reader) (*CachedReader, error) {
	// force the creation of the necessary buckets if we're dealing with an empty db
	if err := cache.EnsureBuckets(db); err != nil {
		return nil, fmt.Errorf("failed to create cache buckets: %w", err)
	}

	// create an error group for managing the processing loop
	eg := &errgroup.Group{}

	r := &CachedReader{
		db:        db,
		batchSize: batchSize,
		delegate:  delegate,
		log:       log.WithPrefix("walk[cache]"),
		eg:        eg,
		updateCh:  make(chan *File, batchSize*runtime.NumCPU()),
	}

	// start the processing loop
	eg.Go(r.process)

	return r, nil
}
