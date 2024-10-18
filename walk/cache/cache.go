package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/adrg/xdg"
	bolt "go.etcd.io/bbolt"
)

const (
	bucketPaths = "paths"
)

func Open(root string) (*bolt.DB, error) {
	var (
		err  error
		path string
	)

	// Otherwise, the database will be located in `XDG_CACHE_DIR/treefmt/eval-cache/<name>.db`, where <name> is
	// determined by hashing the treeRoot path.
	// This associates a given treeRoot with a given instance of the cache.
	digest := sha256.Sum256([]byte(root))

	name := hex.EncodeToString(digest[:])
	if path, err = xdg.CacheFile(fmt.Sprintf("treefmt/eval-cache/%v.db", name)); err != nil {
		return nil, fmt.Errorf("could not resolve local path for the cache: %w", err)
	}

	// open db
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, err
	}

	// ensure bucket exist
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketPaths))

		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create bucket: %w", err)
	}

	return db, nil
}

func PathsBucket(tx *bolt.Tx) *bolt.Bucket {
	return tx.Bucket([]byte("paths"))
}

func deleteAll(bucket *bolt.Bucket) error {
	c := bucket.Cursor()
	for k, v := c.First(); !(k == nil && v == nil); k, v = c.Next() {
		if err := c.Delete(); err != nil {
			return fmt.Errorf("failed to remove cache entry for key %s: %w", string(k), err)
		}
	}

	return nil
}

func Clear(db *bolt.DB) error {
	return db.Update(func(tx *bolt.Tx) error {
		return deleteAll(PathsBucket(tx))
	})
}
