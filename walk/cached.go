package walk

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"

	"github.com/charmbracelet/log"
	"github.com/numtide/treefmt/v2/walk/cache"
	bolt "go.etcd.io/bbolt"
	"golang.org/x/sync/errgroup"
)

type ctxKeyNoCache struct{}

func SetNoCache(ctx context.Context, noCache bool) context.Context {
	return context.WithValue(ctx, ctxKeyNoCache{}, noCache)
}

func GetNoCache(ctx context.Context) bool {
	noCache, ok := ctx.Value(ctxKeyNoCache{}).(bool)

	return ok && noCache
}

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
	batch := make([]*File, 0, c.batchSize)

	flush := func() error {
		// check for an empty batch
		if len(batch) == 0 {
			return nil
		}

		return c.db.Update(func(tx *bolt.Tx) error {
			bucket := cache.PathsBucket(tx)

			// for each file in the batch, calculate its new format signature and update the bucket entry
			for _, file := range batch {
				signature, err := file.NewFormatSignature()
				if err != nil {
					return fmt.Errorf("failed to calculate signature for path %s: %w", file.RelPath, err)
				}

				if err := bucket.Put([]byte(file.RelPath), signature); err != nil {
					return fmt.Errorf("failed to put format signature for path %s: %w", file.RelPath, err)
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

			// reset the batch
			batch = batch[:0]
		}
	}

	// flush final partial batch
	return flush()
}

func (c *CachedReader) Read(ctx context.Context, files []*File) (n int, err error) {
	err = c.db.View(func(tx *bolt.Tx) error {
		// get paths bucket
		bucket := cache.PathsBucket(tx)

		if err != nil {
			return fmt.Errorf("failed to get bucket: %w", err)
		}

		// perform a read on the underlying reader
		n, err = c.delegate.Read(ctx, files)
		c.log.Debugf("read %d files from delegate", n)

		for i := range n {
			file := files[i]

			file.CachedFormatSignature = bucket.Get([]byte(file.RelPath))

			// set a release function which inserts this file into the update channel
			file.AddReleaseFunc(func(ctx context.Context) error {
				if !GetNoCache(ctx) {
					c.updateCh <- file
				}

				return nil
			})
		}

		if errors.Is(err, io.EOF) {
			return err //nolint:wrapcheck
		} else if err != nil {
			return fmt.Errorf("failed to read files from delegate: %w", err)
		}

		return nil
	})

	return n, err //nolint:wrapcheck
}

// Close waits for any processing to complete.
func (c *CachedReader) Close() error {
	// close the release channel
	close(c.updateCh)

	// wait for any pending releases to be processed
	err := c.eg.Wait()
	if err != nil {
		return fmt.Errorf("failed to wait for processing to complete: %w", err)
	}

	return nil
}

// NewCachedReader creates a cache Reader instance, backed by a bolt DB and delegating reads to delegate.
func NewCachedReader(db *bolt.DB, batchSize int, delegate Reader) (*CachedReader, error) {
	eg := &errgroup.Group{} // create an error group for managing the processing loop

	r := &CachedReader{
		db:        db,
		batchSize: batchSize,
		delegate:  delegate,
		log:       log.WithPrefix("walk | cache"),
		eg:        eg,
		updateCh:  make(chan *File, batchSize*runtime.NumCPU()),
	}

	// start the processing loop
	eg.Go(r.process)

	return r, nil
}
