package format

import (
	"fmt"
	"git.numtide.com/numtide/treefmt/cache"
	"github.com/charmbracelet/log"
	"os"
)

func CheckFormatters(c *cache.Cache, formatters map[string]*Formatter) error {
	return c.Update(func(tx *cache.Tx) error {

		clearPaths := false

		pathsBucket, err := tx.Paths()
		if err != nil {
			return fmt.Errorf("failed to get paths bucket from cache: %w", err)
		}

		formattersBucket, err := tx.Formatters()
		if err != nil {
			return fmt.Errorf("failed to get formatters bucket from cache: %w", err)
		}

		// check for any newly configured or modified formatters
		for name, formatter := range formatters {

			stat, err := os.Lstat(formatter.Executable())
			if err != nil {
				return fmt.Errorf("failed to stat formatter executable %v: %w", formatter.Executable(), err)
			}

			entry, err := formattersBucket.Get(name)
			if err != nil {
				return fmt.Errorf("failed to retrieve cache entry for formatter %v: %w", name, err)
			}

			isNew := entry == nil
			hasChanged := entry != nil && !(entry.Size == stat.Size() && entry.Modified == stat.ModTime())

			if isNew {
				log.Debugf("formatter '%s' is new", name)
			} else if hasChanged {
				log.Debug("formatter '%s' has changed",
					name,
					"size", stat.Size(),
					"modTime", stat.ModTime(),
					"cachedSize", entry.Size,
					"cachedModTime", entry.Modified,
				)
			}

			// update overall flag
			clearPaths = clearPaths || isNew || hasChanged

			// record formatters info
			entry = &cache.Entry{
				Size:     stat.Size(),
				Modified: stat.ModTime(),
			}

			if err = formattersBucket.Put(name, entry); err != nil {
				return fmt.Errorf("failed to write cache entry for formatter %v: %w", name, err)
			}
		}

		// check for any removed formatters
		if err = formattersBucket.ForEach(func(key string, _ *cache.Entry) error {
			_, ok := formatters[key]
			if !ok {
				// remove the formatter entry from the cache
				if err = formattersBucket.Delete(key); err != nil {
					return fmt.Errorf("failed to remove cache entry for formatter %v: %w", key, err)
				}
				// indicate a clean is required
				clearPaths = true
			}
			return nil
		}); err != nil {
			return fmt.Errorf("failed to check cache for removed formatters: %w", err)
		}

		if clearPaths {
			// remove all path entries
			if err := pathsBucket.DeleteAll(); err != nil {
				return fmt.Errorf("failed to remove all path entries from cache: %w", err)
			}
		}

		return nil
	})

}
