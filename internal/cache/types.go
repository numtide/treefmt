package cache

import "time"

type FileInfo struct {
	Size     int64
	Modified time.Time
}
