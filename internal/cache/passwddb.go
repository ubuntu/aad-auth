package cache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/ubuntu/aad-auth/internal/pam"
)

// UserRecord returns a user record from the cache
type UserRecord struct {
	Name           string
	Passwd         string
	UID            int
	GID            int
	Gecos          string
	Home           string
	Shell          string
	LastOnlineAuth time.Time

	// if shadow is opened
	ShadowPasswd string
}

// GetUserByName returns given user struct by its name.
// It returns an error if we couldn’t fetch the user (does not exist or not connected).
// shadowPasswd is populated only if the shadow database is accessible.
func (c *Cache) GetUserByName(ctx context.Context, username string) (user UserRecord, err error) {
	pam.LogDebug(ctx, "getting user information from cache for %q", username)

	// This query is dynamically extended whether we have can query the shadow database or not
	queryFmt := `
SELECT login,
	p.password,
	p.uid,
	gid,
	gecos,
	home,
	shell,
	last_online_auth
	%s
FROM   passwd p
%s
WHERE login = ?
%s`

	query := fmt.Sprintf(queryFmt, ",''", "", "")
	if c.hasShadow {
		query = fmt.Sprintf(queryFmt, ",s.password", ",shadow.shadow s", "AND   p.uid = s.uid")
	}

	row := c.db.QueryRow(query, username)
	u, err := newUserFromScanner(row)
	if err != nil {
		return u, fmt.Errorf("error when getting user %q from cache: %w", username, err)
	}

	return u, nil
}

// GetUserByUid returns given user struct by its UID.
// It returns an error if we couldn’t fetch the user (does not exist or not connected).
// shadowPasswd is populated only if the shadow database is accessible.
func (c *Cache) GetUserByUid(ctx context.Context, uid uint) (user UserRecord, err error) {
	pam.LogDebug(ctx, "getting user information from cache for uid %d", uid) // TODO: remove PAM dep

	// This query is dynamically extended whether we have can query the shadow database or not
	queryFmt := `
SELECT login,
	p.password,
	p.uid,
	gid,
	gecos,
	home,
	shell,
	last_online_auth
	%s
FROM   passwd p
%s
WHERE p.uid = ?
%s`

	query := fmt.Sprintf(queryFmt, ",''", "", "")
	if c.hasShadow {
		query = fmt.Sprintf(queryFmt, ",s.password", ",shadow.shadow s", "AND   p.uid = s.uid")
	}

	row := c.db.QueryRow(query, uid)
	u, err := newUserFromScanner(row)
	if err != nil {
		return u, fmt.Errorf("error when getting uid %d from cache: %w", uid, err)
	}

	return u, nil
}

// NextPasswdEntry returns next passwd from the current position within this cache.
// It initializes the passwd query on first run and return ErrNoEnt once done.
func (c *Cache) NextPasswdEntry() (u UserRecord, err error) {
	defer func() {
		if err != nil && !errors.Is(err, ErrNoEnt) {
			err = fmt.Errorf("failed to read passwd entry in db: %v", err)
		}
	}()
	pam.LogDebug(context.Background(), "request next passwd entry in db")

	if c.cursorPasswd == nil {
		query := `
		SELECT login, password, uid, gid, gecos, home, shell, last_online_auth, ''
		FROM passwd
		ORDER BY login`
		c.cursorPasswd, err = c.db.Query(query)
		if err != nil {
			return u, err
		}
	}
	if !c.cursorPasswd.Next() {
		if err := c.cursorPasswd.Close(); err != nil {
			return u, err
		}
		c.cursorPasswd = nil
		return u, ErrNoEnt
	}

	return newUserFromScanner(c.cursorPasswd)
}

// newUserFromScanner abstracts the row request deserialization to UserRecord.
// It returns ErrNoEnt in case of no element found.
func newUserFromScanner(r rowScanner) (u UserRecord, err error) {
	var lastlogin int64
	if err := r.Scan(&u.Name, &u.Passwd, &u.UID, &u.GID, &u.Gecos, &u.Home, &u.Shell, &lastlogin, &u.ShadowPasswd); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = ErrNoEnt
		}
		return UserRecord{}, err
	}

	u.LastOnlineAuth = time.Unix(lastlogin, 0)
	return u, nil
}

//  uidOrGidExists check if uid in passwd or gid in groups does exists.
func uidOrGidExists(db *sql.DB, id uint32, username string) (bool, error) {
	row := db.QueryRow("SELECT login,'',-1, -1,-1,-1,-1,-1,-1 from passwd where uid = ? UNION SELECT name,'',-1, -1,-1,-1,-1,-1,-1 from groups where gid = ?", id, id)

	u, err := newUserFromScanner(row)
	if errors.Is(err, ErrNoEnt) {
		return false, nil
	} else if err != nil {
		return true, fmt.Errorf("failed to verify that %d is unique: %v", id, err)
	}

	// We found one entry, check db inconsistency
	if u.Name == username {
		return true, fmt.Errorf("user already exists in cache")
	}

	return true, nil
}
