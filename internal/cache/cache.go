package cache

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/adrg/xdg"
	"github.com/vmihailenco/msgpack/v5"
	bolt "go.etcd.io/bbolt"
)

const (
	modifiedBucket = "modified"
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
func Open(treeRoot string, clean bool) (err error) {
	// determine a unique and consistent db name for the tree root
	h := sha1.New()
	h.Write([]byte(treeRoot))
	digest := h.Sum(nil)

	name := hex.EncodeToString(digest)
	path, err := xdg.CacheFile(fmt.Sprintf("treefmt/eval-cache/%v.db", name))
	if err != nil {
		return fmt.Errorf("%w: could not resolve local path for the cache", err)
	}

	// force a clean of the cache if specified
	if clean {
		err := os.Remove(path)
		if errors.Is(err, os.ErrNotExist) {
			err = nil
		} else if err != nil {
			return fmt.Errorf("%w: failed to clear cache", err)
		}
	}

	db, err = bolt.Open(path, 0o600, nil)
	if err != nil {
		return fmt.Errorf("%w: failed to open cache", err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket([]byte(modifiedBucket))
		if errors.Is(err, bolt.ErrBucketExists) {
			return nil
		}
		return err
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

// ChangeSet is used to walk a filesystem, starting at root, and outputting any new or changed paths using pathsCh.
// It determines if a path is new or has changed by comparing against cache entries.
func ChangeSet(ctx context.Context, root string, pathsCh chan<- string) error {
	return db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(modifiedBucket))

		return filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return fmt.Errorf("%w: failed to walk path", err)
			} else if ctx.Err() != nil {
				return ctx.Err()
			} else if info.IsDir() {
				// todo what about symlinks?
				return nil
			}

			if info.Mode()&os.ModeSymlink == os.ModeSymlink {
				// skip symlinks
				return nil
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
			pathsCh <- path
			return nil
		})
	})
}

// Update is used to record updated cache information for the specified list of paths.
func Update(paths []string) (int, error) {
	if len(paths) == 0 {
		return 0, nil
	}

	var changes int

	return changes, db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(modifiedBucket))

		for _, path := range paths {
			if path == "" {
				continue
			}

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

			cacheInfo := Entry{
				Size:     pathInfo.Size(),
				Modified: pathInfo.ModTime(),
			}

			bytes, err := msgpack.Marshal(cacheInfo)
			if err != nil {
				return fmt.Errorf("%w: failed to marshal mod time", err)
			}

			if err = bucket.Put([]byte(path), bytes); err != nil {
				return fmt.Errorf("%w: failed to put mode time", err)
			}
		}

		return nil
	})
}
