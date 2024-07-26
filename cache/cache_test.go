package cache

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/adrg/xdg"
	"github.com/stretchr/testify/require"
)

func TestCache_Open(t *testing.T) {
	as := require.New(t)

	tempDir := t.TempDir()
	xdgPrefix, err := xdg.CacheFile("")
	as.NoError(err)

	// normal open
	cache, err := Open(tempDir, false)
	path := cache.db.Path()

	as.NoError(err)
	as.True(
		strings.HasPrefix(path, xdgPrefix),
		"db path %s does not contain the xdg cache file prefix %s",
		path, xdgPrefix,
	)

	// normal close
	as.NoError(cache.Close())
	_, err = os.Stat(path)
	as.NoError(err, "db path %s should still exist after closing the cache", path)

	// open a temp cache e.g. --no-cache
	tempDir = t.TempDir()
	cache, err = Open(tempDir, true)
	as.NoError(err)

	// close temp cache
	as.NoError(cache.Close())
	_, err = os.Stat(cache.db.Path())
	as.ErrorIs(err, os.ErrNotExist, "temp db path %s should not exist after closing the cache")
}

func TestCache_Update(t *testing.T) {
	as := require.New(t)

	cache, err := Open(t.TempDir(), false)
	as.NoError(err)

	now := time.Now()

	testData := map[string]map[string]*Entry{
		"paths": {
			"foo":       {Size: 0, Modified: now},
			"bar":       {Size: 1, Modified: now.Add(-1 * time.Second)},
			"fizz/buzz": {Size: 1 << 16, Modified: now.Add(-1 * time.Minute)},
		},
		"formatters": {
			"bla":         {Size: 1 << 32, Modified: now.Add(-1 * time.Hour)},
			"foo/bar/baz": {Size: 1 << 24, Modified: now.Add(-24 * time.Hour)},
		},
	}

	putEntries := func(bucket *Bucket[Entry], err error) func(string) {
		return func(name string) {
			as.NoError(err)
			for k, v := range testData[name] {
				as.NoError(bucket.Put(k, v), "failed to put value")
			}
		}
	}

	getEntries := func(bucket *Bucket[Entry], err error) func(string) {
		return func(name string) {
			as.NoError(err)
			as.Equal(len(testData[name]), bucket.Size())
			for k, v := range testData[name] {
				actual, err := bucket.Get(k)
				as.NoError(err)
				as.EqualExportedValues(*v, *actual)
			}
		}
	}

	modifyEntries := func(bucket *Bucket[Entry], err error) func(string) {
		return func(name string) {
			entries := testData[name]
			idx := 0
			for k := range entries {
				switch idx {
				case 0:
					// delete the first entry
					as.NoError(bucket.Delete(k))
					delete(entries, k)
				case 1:
					// change the second
					entries[k] = &Entry{Size: 123, Modified: now.Add(-2 * time.Hour)}
					as.NoError(bucket.Put(k, entries[k]))
				default:
					break
				}
			}
		}
	}

	clearEntries := func(bucket *Bucket[Entry], err error) {
		as.NoError(err)
		as.NoError(bucket.DeleteAll())
	}

	checkEmpty := func(bucket *Bucket[Entry], err error) {
		as.NoError(err)
		as.Equal(0, bucket.Size())
	}

	// insert the test data into the cache
	err = cache.Update(func(tx *Tx) error {
		putEntries(tx.Paths())("paths")
		putEntries(tx.Formatters())("formatters")
		return nil
	})
	as.NoError(err)

	// read it back and check it matches
	err = cache.View(func(tx *Tx) error {
		getEntries(tx.Paths())("paths")
		getEntries(tx.Formatters())("formatters")
		return nil
	})
	as.NoError(err)

	// delete and update some entries
	err = cache.Update(func(tx *Tx) error {
		modifyEntries(tx.Paths())
		modifyEntries(tx.Formatters())
		return nil
	})
	as.NoError(err)

	// read them back and make sure they match the updated test data
	err = cache.View(func(tx *Tx) error {
		getEntries(tx.Paths())("paths")
		getEntries(tx.Formatters())("formatters")
		return nil
	})
	as.NoError(err)

	// delete all
	err = cache.Update(func(tx *Tx) error {
		clearEntries(tx.Paths())
		clearEntries(tx.Formatters())
		return nil
	})
	as.NoError(err)

	// check the cache is empty
	err = cache.Update(func(tx *Tx) error {
		checkEmpty(tx.Paths())
		checkEmpty(tx.Formatters())
		return nil
	})
	as.NoError(err)
}
