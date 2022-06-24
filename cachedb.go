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
	gecos				TEXT DEFAULT "",
	home				TEXT DEFAULT "",
	shell				TEXT DEFAULT "/bin/bash",
	last_online_auth 	INTEGER,	-- Last time user has been authenticated against a server
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

func initDB(ctx context.Context, cacheDir string, rootUid, rootGid, shadowGid int) (db *sql.DB, hasShadow bool, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("can’t initiate database: %v", err)
		}
	}()
	pamLogDebug(ctx, "Opening cache in %s", cacheDir)

	passwdPath := filepath.Join(cacheDir, passwdDB)
	var passwdPermission fs.FileMode = 0644
	shadowPath := filepath.Join(cacheDir, shadowDB)
	var shadowPermission fs.FileMode = 0640

	dbFiles := map[string]struct {
		sqlCreate      string
		fileOwner      int
		fileGOwner     int
		filePermission fs.FileMode
	}{
		passwdPath: {sqlCreatePasswdTables, rootUid, rootGid, passwdPermission},
		shadowPath: {sqlCreateShadowTables, rootUid, shadowGid, shadowPermission},
	}

	var needsCreate bool
	for p := range dbFiles {
		if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
			needsCreate = true
		}
	}

	// Ensure that the partial cache (if exists) is cleaned up before creating it
	if needsCreate {
		if os.Geteuid() != rootUid || os.Getegid() != rootGid {
			return nil, false, fmt.Errorf("cache creation can only be done by root user")
		}

		if err := os.RemoveAll(cacheDir); err != nil {
			return nil, false, err
		}
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			return nil, false, err
		}

		for p, prop := range dbFiles {
			db, err := sql.Open("sqlite3", p)
			if err != nil {
				return nil, false, err
			}
			_, err = db.Exec(prop.sqlCreate)
			if err != nil {
				return nil, false, fmt.Errorf("failed to create table: %v", err)
			}
			db.Close()
			if err := os.Chown(p, prop.fileOwner, prop.fileGOwner); err != nil {
				return nil, false, fmt.Errorf("fixing ownership failed: %v", err)
			}
			if err := os.Chmod(p, prop.filePermission); err != nil {
				return nil, false, fmt.Errorf("fixing permission failed: %v", err)
			}
		}
	}

	// Check the cache has expected owner and permissions
	for p, prop := range dbFiles {
		if err := checkFilePermission(ctx, p, prop.fileOwner, prop.fileGOwner, prop.filePermission); err != nil {
			return nil, false, err
		}
	}

	// Open existing cache
	db, err = sql.Open("sqlite3", passwdPath)
	if err != nil {
		return nil, false, err
	}

	// Attach shadow if our user is root or part of the shadow group
	u, err := user.Current()
	if err != nil {
		return nil, false, fmt.Errorf("could not get current user: %v", err)
	}
	grps, err := u.GroupIds()
	if err != nil {
		return nil, false, fmt.Errorf("could not get current user groups: %v", err)
	}
	if os.Geteuid() == rootUid || slices.Contains(grps, "shadow") {
		_, err = db.Exec(fmt.Sprintf("attach database '%s' as shadow;", shadowPath))
		if err != nil {
			return nil, false, err
		}
		hasShadow = true
	}

	return db, hasShadow, nil
}
