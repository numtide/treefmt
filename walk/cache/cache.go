package cache

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/adrg/xdg"
	bolt "go.etcd.io/bbolt"
)

type Entry struct {
	Size     int64
	Modified time.Time
}

func (e *Entry) HasChanged(info fs.FileInfo) bool {
	return !(e.Modified == info.ModTime() && e.Size == info.Size())
}

func Open(root string, temp bool) (*bolt.DB, error) {
	var err error
	var path string

	if temp {
		// If no cache is desired, rather than complicate the logic for cache / no cache, we just create a cache instance
		// backed by a temporary file instead
		if f, err := os.CreateTemp("", "treefmt-no-cache-*.db"); err != nil {
			return nil, fmt.Errorf("failed to create a temporary db file: %w", err)
		} else {
			path = f.Name()
		}
	} else {
		// Otherwise, the database will be located in `XDG_CACHE_DIR/treefmt/eval-cache/<name>.db`, where <name> is
		// determined by hashing the treeRoot path.
		// This associates a given treeRoot with a given instance of the cache.
		h := sha1.New()
		h.Write([]byte(root))
		digest := h.Sum(nil)

		name := hex.EncodeToString(digest)
		if path, err = xdg.CacheFile(fmt.Sprintf("treefmt/eval-cache/%v.db", name)); err != nil {
			return nil, fmt.Errorf("could not resolve local path for the cache: %w", err)
		}
	}

	// open db
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, err
	}

	return db, nil
}

func EnsureBuckets(db *bolt.DB) error {
	// force creation of buckets if they don't already exist
	return db.Update(func(tx *bolt.Tx) error {
		if _, err := BucketPaths(tx); err != nil {
			return err
		}
		_, err := BucketFormatters(tx)
		return err
	})
}

func Clear(db *bolt.DB) error {
	return db.Update(func(tx *bolt.Tx) error {
		bucket, err := BucketPaths(tx)
		if err != nil {
			return fmt.Errorf("failed to get paths bucket: %w", err)
		}
		return bucket.DeleteAll()
	})
}
