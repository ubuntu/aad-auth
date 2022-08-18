package cache

import (
	"context"
	"database/sql"
	// needed to embed the sql files for the creation of the cache db.
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	// register sqlite3 as our database driver.
	_ "github.com/mattn/go-sqlite3"
	"github.com/ubuntu/aad-auth/internal/logger"
	"golang.org/x/exp/slices"
)

const (
	defaultCachePath = "/var/lib/aad/cache"
	passwdDB         = "passwd.db" // root:root 644
	shadowDB         = "shadow.db" // root:shadow 640
)

var (
	//go:embed db/passwd.sql
	sqlCreatePasswdTables string

	//go:embed db/shadow.sql
	sqlCreateShadowTables string
)

type rowScanner interface {
	Scan(...any) error
}

func initDB(ctx context.Context, cacheDir string, rootUID, rootGID, shadowGID, forceShadowMode int, passwdPermission, shadowPermission fs.FileMode) (db *sql.DB, shadowMode int, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("can't initiate database: %w", err)
		}
	}()
	logger.Debug(ctx, "Opening cache in %s", cacheDir)

	passwdPath := filepath.Join(cacheDir, passwdDB)
	shadowPath := filepath.Join(cacheDir, shadowDB)

	dbFiles := map[string]struct {
		sqlCreate      string
		fileOwner      int
		fileGOwner     int
		filePermission fs.FileMode
	}{
		passwdPath: {sqlCreatePasswdTables, rootUID, rootGID, passwdPermission},
		shadowPath: {sqlCreateShadowTables, rootUID, shadowGID, shadowPermission},
	}

	var needsCreate bool
	for p := range dbFiles {
		if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
			needsCreate = true
		}
	}

	// Ensure that the partial cache (if exists) is cleaned up before creating it
	if needsCreate {
		if os.Geteuid() != rootUID || os.Getegid() != rootGID {
			return nil, 0, fmt.Errorf("cache creation can only be done by root user")
		}

		if err := os.RemoveAll(cacheDir); err != nil {
			return nil, 0, err
		}
		// #nosec: G301 - passwd file should be readable. Shadow permissions are handled separately.
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			return nil, 0, err
		}

		for p, prop := range dbFiles {
			db, err := sql.Open("sqlite3", p)
			if err != nil {
				return nil, 0, err
			}
			_, err = db.Exec(prop.sqlCreate)
			if err != nil {
				return nil, 0, fmt.Errorf("failed to create table: %w", err)
			}
			db.Close()
			if err := os.Chown(p, prop.fileOwner, prop.fileGOwner); err != nil {
				return nil, 0, fmt.Errorf("fixing ownership failed: %w", err)
			}
			if err := os.Chmod(p, prop.filePermission); err != nil {
				return nil, 0, fmt.Errorf("fixing permission failed: %w", err)
			}
		}
	}

	// Check the cache has expected owner and permissions
	for p, prop := range dbFiles {
		if err := checkFilePermission(ctx, p, prop.fileOwner, prop.fileGOwner, prop.filePermission); err != nil {
			return nil, 0, err
		}
	}

	// Open existing cache
	db, err = sql.Open("sqlite3", passwdPath)
	if err != nil {
		return nil, 0, err
	}

	// Attach shadow if our user has access to the file (even read-only)
	shadowMode = forceShadowMode
	if forceShadowMode == -1 {
		shadowMode = 0
		if f, err := os.OpenFile(shadowPath, os.O_RDWR, 0); err == nil {
			f.Close()
			shadowMode = shadowRWMode
		} else if f, err := os.Open(shadowPath); err == nil {
			f.Close()
			shadowMode = shadowROMode
		}
	}
	if shadowMode > shadowNotAvailableMode {
		_, err = db.Exec(fmt.Sprintf("attach database '%s' as shadow;", shadowPath))
		if err != nil {
			return nil, 0, err
		}
	}

	return db, shadowMode, nil
}

// insertUser insert newUser in cache databases.
func (c *Cache) insertUser(ctx context.Context, newUser UserRecord) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to insert user %q in local cache: %w", newUser.Name, err)
		}
	}()
	logger.Debug(ctx, "inserting in cache user %q", newUser.Name)

	if c.shadowMode != shadowRWMode {
		return fmt.Errorf("shadow database is not accessible for writing: %v", c.shadowMode)
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
func (c *Cache) updateOnlineAuthAndPassword(ctx context.Context, uid int64, username, shadowPasswd string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to update user %q in local cache: %w", username, err)
		}
	}()
	logger.Debug(ctx, "updating from last online login information for user %q", username)

	if c.shadowMode != shadowRWMode {
		return fmt.Errorf("shadow database is not accessible for writing: %v", c.shadowMode)
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

func cleanUpDB(ctx context.Context, db *sql.DB, offlineCredentialsExpiration time.Duration) error {
	if offlineCredentialsExpiration == 0 {
		logger.Debug(ctx, "Do not clean up database as revalidation period is set to 0")
		return nil
	}

	logger.Debug(ctx, "Clean up database")

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // The rollback will be ignored if the tx has been committed later in the function.

	offlineCredentialsExpirationTime := time.Now().Add(-offlineCredentialsExpiration).Unix()

	// Shadow cleanup
	if _, err := tx.Exec("DELETE FROM shadow.shadow WHERE uid IN (SELECT uid FROM passwd WHERE last_online_auth < ?)", offlineCredentialsExpirationTime); err != nil {
		return err
	}
	// uid_gid cleanup
	if _, err := tx.Exec("DELETE FROM uid_gid WHERE uid IN (SELECT uid FROM passwd WHERE last_online_auth < ?)", offlineCredentialsExpirationTime); err != nil {
		return err
	}
	// passwd cleanup
	if _, err := tx.Exec("DELETE FROM passwd WHERE last_online_auth < ?", offlineCredentialsExpirationTime); err != nil {
		return err
	}
	// empty groups cleanup
	if _, err := tx.Exec("DELETE FROM groups WHERE gid NOT IN (SELECT DISTINCT gid FROM uid_gid)"); err != nil {
		return err
	}

	return tx.Commit()
}

/*func updateUid()   {}
func updateGid()   {}*/
// TODO: add user to local groups.

// UpdateUserAttribute updates an attribute to a specified value for a given user.
// If the attribute is not permitted or the value is invalid, an error is returned.
func (c *Cache) UpdateUserAttribute(ctx context.Context, login, attr string, value any) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("could not update %s for %s: %w", attr, login, err)
		}
	}()

	if !slices.Contains(PasswdUpdateAttributes, attr) {
		return errors.New("invalid attribute")
	}

	logger.Debug(ctx, "Updating %s for user %s", attr, login)

	if b, err := loginExists(c.db, login); !b {
		return errors.New("user does not exist")
	} else if err != nil {
		return err
	}

	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// We control the attribute to update so sanitization for it can be bypassed.
	if _, err = tx.Exec(fmt.Sprintf("UPDATE passwd SET %s = ? WHERE login = ?", attr), value, login); err != nil {
		return err
	}

	return tx.Commit()
}

// QueryUserAttribute searches the passwd table for the given attribute for a user.
// If no attribute is provided, the entire row is returned.
func (c *Cache) QueryUserAttribute(ctx context.Context, login, attr string) (value string, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("could not query %s for %s: %w", attr, login, err)
		}
	}()

	if !slices.Contains(PasswdQueryAttributes, attr) {
		return "", errors.New("invalid attribute")
	}

	logger.Debug(ctx, "Querying %s for user %s", attr, login)

	if b, err := loginExists(c.db, login); !b {
		return "", errors.New("user does not exist")
	} else if err != nil {
		return "", err
	}

	// We control the attribute to query so sanitization for it can be bypassed.
	row := c.db.QueryRow(fmt.Sprintf("SELECT %s from passwd WHERE login = ?", attr), login)
	if err := row.Scan(&value); err != nil {
		return "", fmt.Errorf("cannot scan value: %w", err)
	}

	return value, nil
}
