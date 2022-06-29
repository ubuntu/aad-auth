package cache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/ubuntu/aad-auth/internal/pam"
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
	pam.LogDebug(ctx, "Opening cache in %s", cacheDir)

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

	// Attach shadow if our user has access to the file (even read-only)
	if f, err := os.Open(shadowPath); err == nil {
		f.Close()
		_, err = db.Exec(fmt.Sprintf("attach database '%s' as shadow;", shadowPath))
		if err != nil {
			return nil, false, err
		}
		hasShadow = true
	}

	return db, hasShadow, nil
}

// GetUserByName returns given user struct by its name.
// It returns an error if we couldn’t fetch the user (does not exist or not connected).
// shadowPasswd is populated only if the shadow database is accessible.
func (c *Cache) GetUserByName(ctx context.Context, username string) (user UserRecord, err error) {
	pam.LogDebug(ctx, "getting user information from cache for %q", username)

	// This query is dynamically extended whether we have can query the shadow database or not
	queryFmt := `
SELECT login,
	%s,
	p.uid,
	gid,
	gecos,
	home,
	shell,
	last_online_auth
FROM   passwd p
%s
WHERE login = ?
%s`

	query := fmt.Sprintf(queryFmt, "'' as password", "", "")
	if c.hasShadow {
		query = fmt.Sprintf(queryFmt, "s.password", ", shadow.shadow s", "AND   p.uid = s.uid")
	}

	var u UserRecord
	var lastlogin int64
	row := c.db.QueryRow(query, username)
	if err := row.Scan(&u.Name, &u.ShadowPasswd, &u.UID, &u.GID, &u.Gecos, &u.Home, &u.Shell, &lastlogin); err != nil {
		return u, fmt.Errorf("error when getting user %q from cache: %w", username, err)
	}

	u.LastOnlineAuth = time.Unix(lastlogin, 0)

	return u, nil
}

// GetUserByUid returns given user struct by its UID.
// It returns an error if we couldn’t fetch the user (does not exist or not connected).
// shadowPasswd is populated only if the shadow database is accessible.
func (c *Cache) GetUserByUid(ctx context.Context, uid uint) (user UserRecord, err error) {
	pam.LogDebug(ctx, "getting user information from cache for uid %d", uid)

	// This query is dynamically extended whether we have can query the shadow database or not
	queryFmt := `
SELECT login,
	%s,
	p.uid,
	gid,
	gecos,
	home,
	shell,
	last_online_auth
FROM   passwd p
%s
WHERE p.uid = ?
%s`

	query := fmt.Sprintf(queryFmt, "'' as password", "", "")
	if c.hasShadow {
		query = fmt.Sprintf(queryFmt, "s.password", ", shadow.shadow s", "AND   p.uid = s.uid")
	}

	var u UserRecord
	var lastlogin int64
	row := c.db.QueryRow(query, uid)
	if err := row.Scan(&u.Name, &u.ShadowPasswd, &u.UID, &u.GID, &u.Gecos, &u.Home, &u.Shell, &lastlogin); err != nil {
		return u, fmt.Errorf("error when getting uid %d from cache: %w", uid, err)
	}

	u.LastOnlineAuth = time.Unix(lastlogin, 0)

	return u, nil
}

// insertUser insert newUser in cache databases.
func (c *Cache) insertUser(ctx context.Context, newUser UserRecord) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to insert user %q in local cache: %v", newUser.Name, err)
		}
	}()
	pam.LogDebug(ctx, "inserting in cache user %q", newUser.Name)

	if !c.hasShadow {
		return errors.New("shadow database is not accessible")
	}

	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // The rollback will be ignored if the tx has been committed later in the function.

	lastLoginAuth := newUser.LastOnlineAuth.Unix()
	// passwd table
	if _, err = tx.Exec("INSERT INTO passwd (login, uid, gid, home, shell, last_online_auth) VALUES(?,?,?,?,?,?)",
		newUser.Name, newUser.UID, newUser.GID, newUser.Home, newUser.Shell, lastLoginAuth); err != nil {
		return err
	}
	// shadow db table
	if _, err = tx.Exec("INSERT INTO shadow.shadow (uid, password) VALUES (?,?)",
		newUser.UID, newUser.ShadowPasswd); err != nil {
		return err
	}
	// groups table
	if _, err = tx.Exec("INSERT INTO groups (name, gid) VALUES (?,?)",
		newUser.Name, newUser.GID); err != nil {
		return err
	}
	// uid <-> group pivot table
	if _, err = tx.Exec("INSERT INTO uid_gid (uid, gid) VALUES (?,?)",
		newUser.UID, newUser.GID); err != nil {
		return err
	}

	return tx.Commit()
}

// updateOnlineAuthAndPassword updates password and last_online_auth.
func (c *Cache) updateOnlineAuthAndPassword(ctx context.Context, uid int, username, shadowPasswd string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to update user %q in local cache: %v", username, err)
		}
	}()
	pam.LogDebug(ctx, "updating from last online login information for user %q", username)

	if !c.hasShadow {
		return errors.New("shadow database is not accessible")
	}

	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // The rollback will be ignored if the tx has been committed later in the function.

	if _, err = tx.Exec("UPDATE passwd SET last_online_auth = ? WHERE uid = ?", time.Now().Unix(), uid); err != nil {
		return err
	}
	if _, err = tx.Exec("UPDATE shadow.shadow SET password = ? WHERE uid = ?", shadowPasswd, uid); err != nil {
		return err
	}

	return tx.Commit()
}

func cleanUpDB(ctx context.Context, db *sql.DB, revalidationPeriodDuration time.Duration) error {
	if revalidationPeriodDuration == 0 {
		pam.LogDebug(ctx, "Do not clean up database as revalidation period is set to 0")
		return nil
	}

	pam.LogDebug(ctx, "Clean up database")

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // The rollback will be ignored if the tx has been committed later in the function.

	revalidationTime := time.Now().Add(-revalidationPeriodDuration).Unix()

	// Shadow cleanup
	if _, err := tx.Exec("DELETE FROM shadow.shadow WHERE uid IN (SELECT uid FROM passwd WHERE last_online_auth < ?)", revalidationTime); err != nil {
		return err
	}
	// uid_gid cleanup
	if _, err := tx.Exec("DELETE FROM uid_gid WHERE uid IN (SELECT uid FROM passwd WHERE last_online_auth < ?)", revalidationTime); err != nil {
		return err
	}
	// passwd cleanup
	if _, err := tx.Exec("DELETE FROM passwd WHERE last_online_auth < ?", revalidationTime); err != nil {
		return err
	}
	// empty groups cleanup
	if _, err := tx.Exec("DELETE FROM groups WHERE gid NOT IN (SELECT DISTINCT gid FROM uid_gid)"); err != nil {
		return err
	}

	return tx.Commit()
}

//  uidOrGidExists check if uid in passwd or gid in groups does exists.
func uidOrGidExists(db *sql.DB, id uint32, username string) (bool, error) {
	row := db.QueryRow("SELECT login from passwd where uid = ? UNION SELECT name from groups where gid = ?", id, id)

	var login string
	if err := row.Scan(&login); errors.Is(err, sql.ErrNoRows) {
		return false, nil
	} else if err != nil {
		return true, fmt.Errorf("failed to verify that %d is unique: %v", id, err)
	}

	// We found one entry, check db inconsistency
	if login == username {
		return true, fmt.Errorf("user already exists in cache")
	}

	return true, nil
}

/*func updateUid()   {}
func updateGid()   {}*/
// TODO: add user to local groups
func updateShell() {}
func updateHome()  {}
