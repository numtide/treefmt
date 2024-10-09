package cache

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/fs"
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

func Open(root string) (*bolt.DB, error) {
	var err error
	var path string

	// Otherwise, the database will be located in `XDG_CACHE_DIR/treefmt/eval-cache/<name>.db`, where <name> is
	// determined by hashing the treeRoot path.
	// This associates a given treeRoot with a given instance of the cache.
	digest := sha1.Sum([]byte(root))

	name := hex.EncodeToString(digest[:])
	if path, err = xdg.CacheFile(fmt.Sprintf("treefmt/eval-cache/%v.db", name)); err != nil {
		return nil, fmt.Errorf("could not resolve local path for the cache: %w", err)
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
