// Package filecache memoises values derived from a file — its parsed
// template, its directives, its attribute index — for the life of the
// process, so each template is parsed once however many checks read
// it.
package filecache

import (
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Cache holds the values computed for each file. The zero value is
// ready to use.
type Cache struct {
	mu    sync.Mutex
	files map[string]*file
}

type file struct {
	mod  time.Time
	size int64
	vals map[string]any
}

// Value returns the value stored under key for filename, computing it
// with compute on first use. Values are dropped when the file's
// modification time or size changes. A file that can't be stat'ed isn't
// cached.
func (c *Cache) Value(filename, key string, compute func() any) any {
	abs, err := filepath.Abs(filename)
	if err != nil {
		return compute()
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return compute()
	}
	c.mu.Lock()
	f := c.files[abs]
	if f == nil || !f.mod.Equal(fi.ModTime()) || f.size != fi.Size() {
		f = &file{mod: fi.ModTime(), size: fi.Size(), vals: map[string]any{}}
		if c.files == nil {
			c.files = map[string]*file{}
		}
		c.files[abs] = f
	}
	v, ok := f.vals[key]
	c.mu.Unlock()
	if ok {
		return v
	}
	// Computed outside the lock: compute may itself read other values
	// of the same file.
	v = compute()
	c.mu.Lock()
	f.vals[key] = v
	c.mu.Unlock()
	return v
}
