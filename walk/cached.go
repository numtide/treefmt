package walk

import (
	"context"
	"fmt"
	"runtime"
	"sync/atomic"

	"github.com/charmbracelet/log"
	"github.com/numtide/treefmt/walk/cache"
	bolt "go.etcd.io/bbolt"
	"golang.org/x/sync/errgroup"
)

type CachedReader struct {
	db        *bolt.DB
	log       *log.Logger
	batchSize int
	delegate  Reader

	eg        *errgroup.Group
	count     *atomic.Int32
	releaseCh chan *File
}

func (c *CachedReader) process() error {
	var batch []*File

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		return c.db.Update(func(tx *bolt.Tx) error {
			bucket, err := cache.BucketPaths(tx)
			if err != nil {
				return fmt.Errorf("failed to get bucket: %w", err)
			}

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

	for file := range c.releaseCh {
		batch = append(batch, file)
		if len(batch) == c.batchSize {
			if err := flush(); err != nil {
				return err
			}
			batch = batch[:0]
		}
	}

	return flush()
}

func (c *CachedReader) Read(ctx context.Context, files []*File) (n int, err error) {
	err = c.db.View(func(tx *bolt.Tx) error {
		bucket, err := cache.BucketPaths(tx)
		if err != nil {
			return fmt.Errorf("failed to get bucket: %w", err)
		}

		n, err = c.delegate.Read(ctx, files)
		if err != nil {
			return fmt.Errorf("failed to read files from delegate: %w", err)
		}

		c.log.Debugf("read %d files", n)
		if n == 0 {
			return nil
		}

		for i := 0; i < n; i++ {
			file := files[i]

			file.CacheEntry, err = bucket.Get(file.RelPath)
			if err != nil {
				return err
			}

			file.Release = func() {
				c.releaseCh <- file
			}
		}

		return nil
	})

	return n, err
}

func (c *CachedReader) Close() error {
	// close the release channel
	close(c.releaseCh)

	// wait for any pending releases to be processed
	if err := c.eg.Wait(); err != nil {
		c.log.Errorf("failed to wait for pending releases: %s", err)
	}

	// close db
	return c.db.Close()
}

func NewCachedReader(db *bolt.DB, batchSize int, delegate Reader) (*CachedReader, error) {
	if err := cache.EnsureBuckets(db); err != nil {
		return nil, fmt.Errorf("failed to create cache buckets: %w", err)
	}

	eg := &errgroup.Group{}

	r := &CachedReader{
		db:        db,
		batchSize: batchSize,
		delegate:  delegate,
		log:       log.WithPrefix("walk[cache]"),
		eg:        eg,
		count:     new(atomic.Int32),
		releaseCh: make(chan *File, batchSize*runtime.NumCPU()),
	}

	eg.Go(r.process)

	return r, nil
}
