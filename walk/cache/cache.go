package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/adrg/xdg"
	bolt "go.etcd.io/bbolt"
)

const (
	bucketPaths = "paths"
)

// Path returns a unique local cache file path for the given root string, using its SHA-256 hash.
func Path(root string) (string, error) {
	digest := sha256.Sum256([]byte(root))

	name := hex.EncodeToString(digest[:])

	path, err := xdg.CacheFile(fmt.Sprintf("treefmt/eval-cache/%v.db", name))
	if err != nil {
		return "", fmt.Errorf("could not resolve local path for the cache: %w", err)
	}

	return path, nil
}

// Open initialises and opens a Bolt database for the specified root path.
// Returns a pointer to the opened database or an error if initialisation fails.
func Open(root string) (*bolt.DB, error) {
	// determine the db location
	path, err := Path(root)
	if err != nil {
		return nil, err
	}

	// open db
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open cache db at %s: %w", path, err)
	}

	// ensure bucket exist
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketPaths))
		if err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create bucket: %w", err)
	}

	return db, nil
}

func PathsBucket(tx *bolt.Tx) *bolt.Bucket {
	return tx.Bucket([]byte("paths"))
}

func Remove(root string) error {
	// determine the db location
	path, err := Path(root)
	if err != nil {
		return err
	}

	// Remove any db which might already exist.
	// If a treefmt process is currently running with a db open at the same location, it will continue to function
	// as normal, however, when it exits the disk space its inode was referencing will be reclaimed.
	// This will not work on Windows if we ever support it.
	if err = os.Remove(path); !(err == nil || os.IsNotExist(err)) {
		return fmt.Errorf("failed to remove cache db at %s: %w", path, err)
	}

	return nil
}
