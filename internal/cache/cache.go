package cache

import (
	"context"
	"crypto/sha1"
	"encoding/base32"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/juju/errors"
	"github.com/vmihailenco/msgpack/v5"
	bolt "go.etcd.io/bbolt"
)

const (
	modifiedBucket = "modified"
)

var db *bolt.DB

func Open(treeRoot string, clean bool) (err error) {
	// determine a unique and consistent db name for the tree root
	h := sha1.New()
	h.Write([]byte(treeRoot))
	digest := h.Sum(nil)

	name := base32.StdEncoding.EncodeToString(digest)
	path, err := xdg.CacheFile(fmt.Sprintf("treefmt/eval-cache/%v.db", name))

	// bust the cache if specified
	if clean {
		err := os.Remove(path)
		if errors.Is(err, os.ErrNotExist) {
			err = nil
		} else if err != nil {
			return errors.Annotate(err, "failed to clear cache")
		}
	}

	if err != nil {
		return errors.Annotate(err, "could not resolve local path for the cache")
	}

	db, err = bolt.Open(path, 0o600, nil)
	if err != nil {
		return errors.Annotate(err, "failed to open cache")
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

func Close() error {
	return db.Close()
}

func ChangeSet(ctx context.Context, root string, pathsCh chan<- string) error {
	return db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(modifiedBucket))

		return filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return errors.Annotate(err, "failed to walk path")
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

			b := bucket.Get([]byte(path))

			var cached FileInfo

			if b != nil {
				if err = msgpack.Unmarshal(b, &cached); err != nil {
					return errors.Annotatef(err, "failed to unmarshal cache info for path '%v'", path)
				}
			}

			changedOrNew := !(cached.Modified == info.ModTime() && cached.Size == info.Size())

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

func WriteModTime(paths []string) error {
	if len(paths) == 0 {
		return nil
	}

	return db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(modifiedBucket))

		for _, path := range paths {
			if path == "" {
				continue
			}
			pathInfo, err := os.Stat(path)
			if err != nil {
				return err
			}

			cacheInfo := FileInfo{
				Size:     pathInfo.Size(),
				Modified: pathInfo.ModTime(),
			}

			bytes, err := msgpack.Marshal(cacheInfo)
			if err != nil {
				return errors.Annotate(err, "failed to marshal mod time")
			}

			if err = bucket.Put([]byte(path), bytes); err != nil {
				return errors.Annotate(err, "failed to put mode time")
			}
		}

		return nil
	})
}
