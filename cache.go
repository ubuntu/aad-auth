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
	"golang.org/x/exp/slices"
)

const (
	defaultCachePath = "/var/lib/aad/cache"
	passwdDB         = "passwd.db" // root:root 644
	shadowDB         = "shadow.db" // root:shadow 640

	sqlCreatePasswdTables = `
CREATE TABLE IF NOT EXISTS passwd (
	login				TEXT NOT NULL UNIQUE,
	password			TEXT DEFAULT 'x',
	uid					INTEGER	NOT NULL UNIQUE,
	gid					INTEGER NOT NULL,
	gecos				TEXT,
	home				TEXT,
	shell				TEXT,
	last_online_auth 	INTEGER,
	PRIMARY KEY("uid")
);
CREATE UNIQUE INDEX idx_login ON passwd ("login");

CREATE TABLE IF NOT EXISTS groups (
	name		TEXT NOT NULL UNIQUE,
	password	TEXT DEFAULT 'x',
	gid			INT NOT NULL UNIQUE,
	PRIMARY KEY("gid")
);
CREATE UNIQUE INDEX "idx_group_name" ON groups ("name");

CREATE TABLE IF NOT EXISTS uid_gid (
	uid	INT NOT NULL,
	gid INT NOT NULL,
	PRIMARY KEY("uid", "gid")
);
CREATE UNIQUE INDEX "idx_ug_gid" ON "uid_gid" ("gid");`

	sqlCreateShadowTables = `CREATE TABLE IF NOT EXISTS shadow (
	uid				INTEGER NOT NULL UNIQUE,
	password		TEXT	NOT NULL,
	last_pwd_change	INTEGER DEFAULT 0,
	min_pwd_age		INTEGER DEFAULT 0,
	max_pwd_age		INTEGER DEFAULT 0,
	pwd_warn_period	INTEGER DEFAULT 0,
	pwd_inactivity	INTEGER DEFAULT 0,
	expiration_date	INTEGER DEFAULT 0,
	PRIMARY KEY("uid")
);`
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
	pamLogDebug(ctx, "Opening cache in %s", o.cacheDir)

	passwdPath := filepath.Join(o.cacheDir, passwdDB)
	var passwdPermission fs.FileMode = 0644
	shadowPath := filepath.Join(o.cacheDir, shadowDB)
	var shadowPermission fs.FileMode = 0640

	dbFiles := map[string]struct {
		sqlCreate      string
		fileOwner      int
		fileGOwner     int
		filePermission fs.FileMode
	}{
		passwdPath: {sqlCreatePasswdTables, o.rootUid, o.rootGid, passwdPermission},
		shadowPath: {sqlCreateShadowTables, o.rootUid, o.shadowGid, shadowPermission},
	}

	var needsCreate bool
	for p := range dbFiles {
		if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
			needsCreate = true
		}
	}

	// Ensure that the partial cache (if exists) is cleaned up before creating it
	if needsCreate {
		if os.Geteuid() != o.rootUid || os.Getegid() != o.rootGid {
			return nil, fmt.Errorf("cache creation can only be done by root user")
		}

		if err := os.RemoveAll(o.cacheDir); err != nil {
			return nil, err
		}
		if err := os.MkdirAll(o.cacheDir, 0755); err != nil {
			return nil, err
		}

		for p, prop := range dbFiles {
			db, err := sql.Open("sqlite3", p)
			if err != nil {
				return nil, err
			}
			_, err = db.Exec(prop.sqlCreate)
			if err != nil {
				return nil, fmt.Errorf("failed to create table: %v", err)
			}
			db.Close()
			if err := os.Chown(p, prop.fileOwner, prop.fileGOwner); err != nil {
				return nil, fmt.Errorf("fixing ownership failed: %v", err)
			}
			if err := os.Chmod(p, prop.filePermission); err != nil {
				return nil, fmt.Errorf("fixing permission failed: %v", err)
			}
		}
	}

	// Check the cache has expected owner and permissions
	for p, prop := range dbFiles {
		if err := checkFilePermission(ctx, p, prop.fileOwner, prop.fileGOwner, prop.filePermission); err != nil {
			return nil, err
		}
	}

	// Open existing cache
	db, err := sql.Open("sqlite3", passwdPath)
	if err != nil {
		return nil, err
	}

	// Attach shadow if our user is root or part of the shadow group
	u, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("could not get current user: %v", err)
	}
	grps, err := u.GroupIds()
	if err != nil {
		return nil, fmt.Errorf("could not get current user groups: %v", err)
	}
	if os.Geteuid() == o.rootUid || slices.Contains(grps, "shadow") {
		_, err = db.Exec(fmt.Sprintf("attach database '%s' as shadow;", shadowPath))
		if err != nil {
			return nil, err
		}
		hasShadow = true
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
