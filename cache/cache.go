package cache

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"time"

	"git.numtide.com/numtide/treefmt/format"
	"git.numtide.com/numtide/treefmt/walk"

	"github.com/charmbracelet/log"

	"github.com/adrg/xdg"
	"github.com/vmihailenco/msgpack/v5"
	bolt "go.etcd.io/bbolt"
)

const (
	pathsBucket      = "paths"
	formattersBucket = "formatters"

	readBatchSize = 1024
)

// Entry represents a cache entry, indicating the last size and modified time for a file path.
type Entry struct {
	Size     int64
	Modified time.Time
}

var db *bolt.DB

// Open creates an instance of bolt.DB for a given treeRoot path.
// If clean is true, Open will delete any existing data in the cache.
//
// The database will be located in `XDG_CACHE_DIR/treefmt/eval-cache/<id>.db`, where <id> is determined by hashing
// the treeRoot path. This associates a given treeRoot with a given instance of the cache.
func Open(treeRoot string, clean bool, formatters map[string]*format.Formatter) (err error) {
	l := log.WithPrefix("cache")

	// determine a unique and consistent db name for the tree root
	h := sha1.New()
	h.Write([]byte(treeRoot))
	digest := h.Sum(nil)

	name := hex.EncodeToString(digest)
	path, err := xdg.CacheFile(fmt.Sprintf("treefmt/eval-cache/%v.db", name))
	if err != nil {
		return fmt.Errorf("%w: could not resolve local path for the cache", err)
	}

	db, err = bolt.Open(path, 0o600, nil)
	if err != nil {
		return fmt.Errorf("%w: failed to open cache", err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		// create bucket for tracking paths
		pathsBucket, err := tx.CreateBucketIfNotExists([]byte(pathsBucket))
		if err != nil {
			return fmt.Errorf("%w: failed to create paths bucket", err)
		}

		// create bucket for tracking formatters
		formattersBucket, err := tx.CreateBucketIfNotExists([]byte(formattersBucket))
		if err != nil {
			return fmt.Errorf("%w: failed to create formatters bucket", err)
		}

		// check for any newly configured or modified formatters
		for name, formatter := range formatters {

			stat, err := os.Lstat(formatter.Executable())
			if err != nil {
				return fmt.Errorf("%w: failed to state formatter executable", err)
			}

			entry, err := getEntry(formattersBucket, name)
			if err != nil {
				return fmt.Errorf("%w: failed to retrieve entry for formatter", err)
			}

			clean = clean || entry == nil || !(entry.Size == stat.Size() && entry.Modified == stat.ModTime())
			l.Debug(
				"checking if formatter has changed",
				"name", name,
				"clean", clean,
				"entry", entry,
				"stat", stat,
			)

			// record formatters info
			entry = &Entry{
				Size:     stat.Size(),
				Modified: stat.ModTime(),
			}

			if err = putEntry(formattersBucket, name, entry); err != nil {
				return fmt.Errorf("%w: failed to write formatter entry", err)
			}
		}

		// check for any removed formatters
		if err = formattersBucket.ForEach(func(key []byte, _ []byte) error {
			_, ok := formatters[string(key)]
			if !ok {
				// remove the formatter entry from the cache
				if err = formattersBucket.Delete(key); err != nil {
					return fmt.Errorf("%w: failed to remove formatter entry", err)
				}
				// indicate a clean is required
				clean = true
			}
			return nil
		}); err != nil {
			return fmt.Errorf("%w: failed to check for removed formatters", err)
		}

		if clean {
			// remove all path entries
			c := pathsBucket.Cursor()
			for k, v := c.First(); !(k == nil && v == nil); k, v = c.Next() {
				if err = c.Delete(); err != nil {
					return fmt.Errorf("%w: failed to remove path entry", err)
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
			return nil, fmt.Errorf("%w: failed to unmarshal cache info for path '%v'", err, path)
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
		return fmt.Errorf("%w: failed to marshal cache entry", err)
	}

	if err = bucket.Put([]byte(path), bytes); err != nil {
		return fmt.Errorf("%w: failed to put cache entry", err)
	}
	return nil
}

// ChangeSet is used to walk a filesystem, starting at root, and outputting any new or changed paths using pathsCh.
// It determines if a path is new or has changed by comparing against cache entries.
func ChangeSet(ctx context.Context, walker walk.Walker, pathsCh chan<- string) error {
	var tx *bolt.Tx
	var bucket *bolt.Bucket
	var processed int

	defer func() {
		// close any pending read tx
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	return walker.Walk(ctx, func(path string, info fs.FileInfo, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err != nil {
				return fmt.Errorf("%w: failed to walk path", err)
			} else if info.IsDir() {
				// ignore directories
				return nil
			}
		}

		// ignore symlinks
		if info.Mode()&os.ModeSymlink == os.ModeSymlink {
			return nil
		}

		// open a new read tx if there isn't one in progress
		// we have to periodically open a new read tx to prevent writes from being blocked
		if tx == nil {
			tx, err = db.Begin(false)
			if err != nil {
				return fmt.Errorf("%w: failed to open a new read tx", err)
			}
			bucket = tx.Bucket([]byte(pathsBucket))
		}

		cached, err := getEntry(bucket, path)
		if err != nil {
			return err
		}

		changedOrNew := cached == nil || !(cached.Modified == info.ModTime() && cached.Size == info.Size())

		if !changedOrNew {
			// no change
			return nil
		}

		// pass on the path
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			pathsCh <- path
		}

		// close the current tx if we have reached the batch size
		processed += 1
		if processed == readBatchSize {
			return tx.Rollback()
		}

		return nil
	})
}

// Update is used to record updated cache information for the specified list of paths.
func Update(paths []string) (int, error) {
	if len(paths) == 0 {
		return 0, nil
	}

	var changes int

	return changes, db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(pathsBucket))

		for _, path := range paths {
			cached, err := getEntry(bucket, path)
			if err != nil {
				return err
			}

			pathInfo, err := os.Stat(path)
			if err != nil {
				return err
			}

			if cached == nil || !(cached.Modified == pathInfo.ModTime() && cached.Size == pathInfo.Size()) {
				changes += 1
			} else {
				// no change to write
				continue
			}

			entry := Entry{
				Size:     pathInfo.Size(),
				Modified: pathInfo.ModTime(),
			}

			if err = putEntry(bucket, path, &entry); err != nil {
				return err
			}
		}

		return nil
	})
}
