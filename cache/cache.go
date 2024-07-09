package cache

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"runtime"
	"time"

	"git.numtide.com/numtide/treefmt/stats"

	"git.numtide.com/numtide/treefmt/format"
	"git.numtide.com/numtide/treefmt/walker"

	"github.com/charmbracelet/log"

	"github.com/adrg/xdg"
	"github.com/vmihailenco/msgpack/v5"
	bolt "go.etcd.io/bbolt"
)

const (
	pathsBucket      = "paths"
	formattersBucket = "formatters"
)

// Entry represents a cache entry, indicating the last size and modified time for a file path.
type Entry struct {
	Size     int64
	Modified time.Time
}

var (
	db     *bolt.DB
	logger *log.Logger

	ReadBatchSize = 1024 * runtime.NumCPU()
)

// Open creates an instance of bolt.DB for a given treeRoot path.
// If clean is true, Open will delete any existing data in the cache.
//
// The database will be located in `XDG_CACHE_DIR/treefmt/eval-cache/<id>.db`, where <id> is determined by hashing
// the treeRoot path. This associates a given treeRoot with a given instance of the cache.
func Open(treeRoot string, clean bool, formatters map[string]*format.Formatter) (err error) {
	logger = log.WithPrefix("cache")

	// determine a unique and consistent db name for the tree root
	h := sha1.New()
	h.Write([]byte(treeRoot))
	digest := h.Sum(nil)

	name := hex.EncodeToString(digest)
	path, err := xdg.CacheFile(fmt.Sprintf("treefmt/eval-cache/%v.db", name))
	if err != nil {
		return fmt.Errorf("could not resolve local path for the cache: %w", err)
	}

	db, err = bolt.Open(path, 0o600, nil)
	if err != nil {
		return fmt.Errorf("failed to open cache at %v: %w", path, err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		// create bucket for tracking paths
		pathsBucket, err := tx.CreateBucketIfNotExists([]byte(pathsBucket))
		if err != nil {
			return fmt.Errorf("failed to create paths bucket: %w", err)
		}

		// create bucket for tracking formatters
		formattersBucket, err := tx.CreateBucketIfNotExists([]byte(formattersBucket))
		if err != nil {
			return fmt.Errorf("failed to create formatters bucket: %w", err)
		}

		// check for any newly configured or modified formatters
		for name, formatter := range formatters {

			stat, err := os.Lstat(formatter.Executable())
			if err != nil {
				return fmt.Errorf("failed to stat formatter executable %v: %w", formatter.Executable(), err)
			}

			entry, err := getEntry(formattersBucket, name)
			if err != nil {
				return fmt.Errorf("failed to retrieve cache entry for formatter %v: %w", name, err)
			}

			isNew := entry == nil
			hasChanged := entry != nil && !(entry.Size == stat.Size() && entry.Modified == stat.ModTime())

			if isNew {
				logger.Debugf("formatter '%s' is new", name)
			} else if hasChanged {
				logger.Debug("formatter '%s' has changed",
					name,
					"size", stat.Size(),
					"modTime", stat.ModTime(),
					"cachedSize", entry.Size,
					"cachedModTime", entry.Modified,
				)
			}

			// update overall clean flag
			clean = clean || isNew || hasChanged

			// record formatters info
			entry = &Entry{
				Size:     stat.Size(),
				Modified: stat.ModTime(),
			}

			if err = putEntry(formattersBucket, name, entry); err != nil {
				return fmt.Errorf("failed to write cache entry for formatter %v: %w", name, err)
			}
		}

		// check for any removed formatters
		if err = formattersBucket.ForEach(func(key []byte, _ []byte) error {
			_, ok := formatters[string(key)]
			if !ok {
				// remove the formatter entry from the cache
				if err = formattersBucket.Delete(key); err != nil {
					return fmt.Errorf("failed to remove cache entry for formatter %v: %w", key, err)
				}
				// indicate a clean is required
				clean = true
			}
			return nil
		}); err != nil {
			return fmt.Errorf("failed to check cache for removed formatters: %w", err)
		}

		if clean {
			// remove all path entries
			c := pathsBucket.Cursor()
			for k, v := c.First(); !(k == nil && v == nil); k, v = c.Next() {
				if err = c.Delete(); err != nil {
					return fmt.Errorf("failed to remove path entry: %w", err)
				}
			}
		}

		return nil
	})

	return
}

// Close closes any open instance of the cache.
func Close() error {
	if db == nil {
		return nil
	}
	return db.Close()
}

// getEntry is a helper for reading cache entries from bolt.
func getEntry(bucket *bolt.Bucket, path string) (*Entry, error) {
	b := bucket.Get([]byte(path))
	if b != nil {
		var cached Entry
		if err := msgpack.Unmarshal(b, &cached); err != nil {
			return nil, fmt.Errorf("failed to unmarshal cache info for path '%v': %w", path, err)
		}
		return &cached, nil
	} else {
		return nil, nil
	}
}

// putEntry is a helper for writing cache entries into bolt.
func putEntry(bucket *bolt.Bucket, path string, entry *Entry) error {
	bytes, err := msgpack.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal cache path %v: %w", path, err)
	}

	if err = bucket.Put([]byte(path), bytes); err != nil {
		return fmt.Errorf("failed to put cache path %v: %w", path, err)
	}
	return nil
}

// ChangeSet is used to walk a filesystem, starting at root, and outputting any new or changed paths using pathsCh.
// It determines if a path is new or has changed by comparing against cache entries.
func ChangeSet(ctx context.Context, wk walker.Walker, filesCh chan<- *walker.File) error {
	start := time.Now()

	defer func() {
		logger.Debugf("finished generating change set in %v", time.Since(start))
	}()

	var tx *bolt.Tx
	var bucket *bolt.Bucket
	var processed int

	defer func() {
		// close any pending read tx
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	return wk.Walk(ctx, func(file *walker.File, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err != nil {
				return fmt.Errorf("failed to walk path: %w", err)
			} else if file.Info.IsDir() {
				// ignore directories
				return nil
			}
		}

		// open a new read tx if there isn't one in progress
		// we have to periodically open a new read tx to prevent writes from being blocked
		if tx == nil {
			tx, err = db.Begin(false)
			if err != nil {
				return fmt.Errorf("failed to open a new cache read tx: %w", err)
			}
			bucket = tx.Bucket([]byte(pathsBucket))
		}

		cached, err := getEntry(bucket, file.RelPath)
		if err != nil {
			return err
		}

		changedOrNew := cached == nil || !(cached.Modified == file.Info.ModTime() && cached.Size == file.Info.Size())

		stats.Add(stats.Traversed, 1)
		if !changedOrNew {
			// no change
			return nil
		}

		stats.Add(stats.Emitted, 1)

		// pass on the path
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			filesCh <- file
		}

		// close the current tx if we have reached the batch size
		processed += 1
		if processed == ReadBatchSize {
			err = tx.Rollback()
			tx = nil
			return err
		}

		return nil
	})
}

// Update is used to record updated cache information for the specified list of paths.
func Update(files []*walker.File) error {
	start := time.Now()
	defer func() {
		logger.Debugf("finished processing %v paths in %v", len(files), time.Since(start))
	}()

	if len(files) == 0 {
		return nil
	}

	return db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(pathsBucket))

		for _, f := range files {
			entry := Entry{
				Size:     f.Info.Size(),
				Modified: f.Info.ModTime(),
			}

			if err := putEntry(bucket, f.RelPath, &entry); err != nil {
				return err
			}
		}

		return nil
	})
}
