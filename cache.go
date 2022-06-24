package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"

	_ "github.com/mattn/go-sqlite3"
)

type cache struct {
	db        *sql.DB
	hasShadow bool
}

type options struct {
	cacheDir  string
	rootUid   int
	rootGid   int
	shadowGid int // this bypass group lookup
}
type option func(*options) error

// WithCacheDir specifies a personalized cache directory.
func WithCacheDir(p string) func(o *options) error {
	return func(o *options) error {
		o.cacheDir = p
		return nil
	}
}

//////////////////// to move for tests

// WithRootUid allows to change current Root Uid for tests
func WithRootUid(uid int) func(o *options) error {
	return func(o *options) error {
		o.rootUid = uid
		return nil
	}
}

// WithRootGid allows to change current Root Guid for tests
func WithRootGid(gid int) func(o *options) error {
	return func(o *options) error {
		o.rootGid = gid
		return nil
	}
}

// WithShadowGid allow change current Shadow Gid for tests
func WithShadowGid(shadowGid int) func(o *options) error {
	return func(o *options) error {
		o.shadowGid = shadowGid
		return nil
	}
}

// NewCache returns a new cache handler with the database opens. The cache should be closed once unused with .Close()
// There are 2 caches files: one for passwd/group and one for shadow.
// If both does not exists, NewCache will create them with proper permissions only if you are the root user, otherwise
// NewCache will fail.
// If the cache exists, root or members of shadow will open passwd/group and shadow database. Other users will only open
// passwd/group.
// Every open will check for cache ownership and validity permission. If it has been tempered, NewCache will fail.
func NewCache(ctx context.Context, opts ...option) (c *cache, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("can't open/create cache: %v", err)
		}
	}()

	pamLogDebug(ctx, "Cache initialization")
	var hasShadow bool

	shadowGrp, err := user.LookupGroup("shadow")
	if err != nil {
		return nil, fmt.Errorf("failed to find group id for group shadow: %v", err)
	}
	shadowGid, err := strconv.Atoi(shadowGrp.Gid)
	if err != nil {
		return nil, fmt.Errorf("failed to read shadow group id: %v", err)
	}

	o := options{
		cacheDir:  defaultCachePath,
		rootUid:   0,
		rootGid:   0,
		shadowGid: shadowGid,
	}
	// applied options
	for _, opt := range opts {
		if err := opt(&o); err != nil {
			return nil, err
		}
	}

	db, hasShadow, err := initDB(ctx, o.cacheDir, o.rootUid, o.rootGid, o.shadowGid)
	if err != nil {
		return nil, err
	}
	pamLogDebug(ctx, "Attaching shadow db: %v", hasShadow)

	return &cache{
		db:        db,
		hasShadow: hasShadow,
	}, nil
}

// Close closes the underlying db.
func (c *cache) Close() error {
	return c.db.Close()
}

// checkFilePermission ensure that the file has correct ownership and permissions.
func checkFilePermission(ctx context.Context, p string, owner, gOwner int, permission fs.FileMode) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed checking file permission for %v: %v", p, err)
		}
	}()
	pamLogDebug(ctx, "check file permissions on %v", p)

	info, err := os.Stat(p)
	if err != nil {
		return err
	}
	if info.Mode() != permission {
		return fmt.Errorf("invalid file permission: %s instead of %s", info.Mode(), permission)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("can't get ownership of file")
	}

	if owner != int(stat.Uid) || gOwner != int(stat.Gid) {
		return fmt.Errorf("invalid ownership: %d:%d instead of %d:%d", owner, gOwner, stat.Uid, stat.Gid)
	}

	return nil
}
