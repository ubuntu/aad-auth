package cache

import (
	"io/fs"
	"time"
)

const (
	PasswdDB = passwdDB
	ShadowDB = shadowDB
)

// Those are var, as we are using their addresses.
var (
	ShadowNotAvailableMode = shadowNotAvailableMode
	ShadowROMode           = shadowROMode
	ShadowRWMode           = shadowRWMode
)

// WithPasswdPermission allows to change default, safe, passwd filemode.
func WithPasswdPermission(perm fs.FileMode) func(o *options) error {
	return func(o *options) error {
		o.passwdPermission = perm
		return nil
	}
}

// WithShadowPermission allows to change default, safe, shadow filemode.
func WithShadowPermission(perm fs.FileMode) func(o *options) error {
	return func(o *options) error {
		o.shadowPermission = perm
		return nil
	}
}

func (c *Cache) WaitForCacheClosed() {
	for {
		openedCachesMu.Lock()
		if _, ok := openedCaches[c.sig]; !ok {
			openedCachesMu.Unlock()
			return
		}
		openedCachesMu.Unlock()
		time.Sleep(time.Millisecond * 100)
	}
}

// SetShadowMode changes the internal recorded state without changing the created files itself.
func (c *Cache) SetShadowMode(shadowMode int) {
	c.shadowMode = shadowMode
}

func (c *Cache) ShadowMode() int {
	return c.shadowMode
}
