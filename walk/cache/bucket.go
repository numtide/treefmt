package cache

import (
	"fmt"

	"github.com/vmihailenco/msgpack/v5"
	bolt "go.etcd.io/bbolt"
)

const (
	bucketPaths      = "paths"
	bucketFormatters = "formatters"
)

var ErrKeyNotFound = fmt.Errorf("key not found")

type Bucket[V any] struct {
	bucket *bolt.Bucket
}

func (b *Bucket[V]) Size() int {
	return b.bucket.Stats().KeyN
}

func (b *Bucket[V]) Get(key string) (*V, error) {
	bytes := b.bucket.Get([]byte(key))
	if bytes == nil {
		return nil, ErrKeyNotFound
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

func BucketPaths(tx *bolt.Tx) (*Bucket[Entry], error) {
	return cacheBucket(bucketPaths, tx)
}

func BucketFormatters(tx *bolt.Tx) (*Bucket[Entry], error) {
	return cacheBucket(bucketFormatters, tx)
}

func cacheBucket(name string, tx *bolt.Tx) (*Bucket[Entry], error) {
	var (
		err error
		b   *bolt.Bucket
	)

	if tx.Writable() {
		b, err = tx.CreateBucketIfNotExists([]byte(name))
	} else {
		b = tx.Bucket([]byte(name))
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get/create bucket %s: %w", bucketPaths, err)
	}

	return &Bucket[Entry]{b}, nil
}
