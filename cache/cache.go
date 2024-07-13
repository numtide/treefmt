package cache

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/adrg/xdg"
	"github.com/charmbracelet/log"
	"github.com/vmihailenco/msgpack/v5"
	bolt "go.etcd.io/bbolt"
)

const (
	bucketPaths      = "paths"
	bucketFormatters = "formatters"
)

type Entry struct {
	Size     int64
	Modified time.Time
}

type Bucket[V any] struct {
	bucket *bolt.Bucket
}

func (b *Bucket[V]) Get(key string) (*V, error) {
	bytes := b.bucket.Get([]byte(key))
	if bytes == nil {
		return nil, nil
	}
	var value V
	if err := msgpack.Unmarshal(bytes, &value); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cache entry for key '%v': %w", key, err)
	}
	return &value, nil
}

func (b *Bucket[V]) Put(key string, value *V) error {
	if bytes, err := msgpack.Marshal(value); err != nil {
		return fmt.Errorf("failed to marshal cache entry for key %v: %w", key, err)
	} else if err = b.bucket.Put([]byte(key), bytes); err != nil {
		return fmt.Errorf("failed to put cache entry for key %v: %w", key, err)
	}
	return nil
}

func (b *Bucket[V]) Delete(key string) error {
	return b.bucket.Delete([]byte(key))
}

func (b *Bucket[V]) DeleteAll() error {
	c := b.bucket.Cursor()
	for k, v := c.First(); !(k == nil && v == nil); k, v = c.Next() {
		if err := c.Delete(); err != nil {
			return fmt.Errorf("failed to remove cache entry for key %s: %w", string(k), err)
		}
	}
	return nil
}

func (b *Bucket[V]) ForEach(f func(string, *V) error) error {
	return b.bucket.ForEach(func(key, bytes []byte) error {
		var value V
		if err := msgpack.Unmarshal(bytes, &value); err != nil {
			return fmt.Errorf("failed to unmarshal cache entry for key '%v': %w", key, err)
		}
		return f(string(key), &value)
	})
}

type Tx struct {
	tx *bolt.Tx
}

func (t *Tx) Commit() error {
	return t.tx.Commit()
}

func (t *Tx) Rollback() error {
	return t.tx.Rollback()
}

func (t *Tx) Paths() (*Bucket[Entry], error) {
	return t.cacheBucket(bucketPaths)
}

func (t *Tx) Formatters() (*Bucket[Entry], error) {
	return t.cacheBucket(bucketFormatters)
}

func (t *Tx) cacheBucket(name string) (*Bucket[Entry], error) {
	var b *bolt.Bucket
	var err error
	if t.tx.Writable() {
		b, err = t.tx.CreateBucketIfNotExists([]byte(name))
	} else {
		b = t.tx.Bucket([]byte(name))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get/create bucket %s: %w", bucketPaths, err)
	}
	return &Bucket[Entry]{b}, nil
}

type Cache struct {
	db        *bolt.DB
	Temporary bool
}

func (c *Cache) BeginTx(writeable bool) (*Tx, error) {
	tx, err := c.db.Begin(writeable)
	return &Tx{tx}, err
}

func (c *Cache) View(f func(*Tx) error) error {
	return c.db.View(func(tx *bolt.Tx) error {
		return f(&Tx{tx})
	})
}

func (c *Cache) Update(f func(*Tx) error) error {
	return c.db.Update(func(tx *bolt.Tx) error {
		return f(&Tx{tx})
	})
}

func (c *Cache) Close() error {
	path := c.db.Path()

	// if this is a temporary cache instance, clean up the db file after closing
	if c.Temporary {
		defer func() {
			if err := os.Remove(path); err != nil {
				log.Errorf("failed to remove temporary cache file %s: %v", path, err)
			}
			log.Debugf("successfully removed temporary cache file %s", path)
		}()
	}

	return c.db.Close()
}

func Open(treeRoot string, noCache bool) (*Cache, error) {
	var err error
	var path string

	if noCache {
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
		h.Write([]byte(treeRoot))
		digest := h.Sum(nil)

		name := hex.EncodeToString(digest)
		if path, err = xdg.CacheFile(fmt.Sprintf("treefmt/eval-cache/%v.db", name)); err != nil {
			return nil, fmt.Errorf("could not resolve local path for the cache: %w", err)
		}
	}

	// open db
	db, err := bolt.Open(path, 0o600, nil)
	if err != nil {
		return nil, err
	}

	c := &Cache{
		db:        db,
		Temporary: noCache,
	}

	// force creation of buckets if they don't already exist
	return c, c.Update(func(tx *Tx) error {
		if _, err := tx.Paths(); err != nil {
			return err
		}
		_, err = tx.Formatters()
		return err
	})
}
