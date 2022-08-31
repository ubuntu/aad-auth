package cache

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-ini/ini"
	"github.com/ubuntu/aad-auth/internal/i18n"
	"github.com/ubuntu/aad-auth/internal/logger"
)

// UserRecord returns a user record from the cache.
type UserRecord struct {
	Name           string    `ini:"login"`
	Passwd         string    `ini:"password"`
	UID            int64     `ini:"uid"`
	GID            int64     `ini:"gid"`
	Gecos          string    `ini:"gecos"`
	Home           string    `ini:"home"`
	Shell          string    `ini:"shell"`
	LastOnlineAuth time.Time `ini:"last_online_auth"`

	// if shadow is opened
	ShadowPasswd string `ini:"shadow_password"`
}

// IniString returns an ini representation of the user record as a string.
func (u UserRecord) IniString() (string, error) {
	buf := new(bytes.Buffer)
	out := ini.Empty()
	if err := ini.ReflectFrom(out, &u); err != nil {
		return "", err
	}

	if _, err := out.WriteTo(buf); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// PasswdQueryAttributes returns a list of attributes that can be queried in the
// passwd table.
var PasswdQueryAttributes = []string{
	"login",
	"password",
	"uid",
	"gid",
	"gecos",
	"home",
	"shell",
	"last_online_auth",
}

// PasswdUpdateAttributes returns a list of attributes that can be modified in
// the passwd table.
var PasswdUpdateAttributes = []string{
	"gecos",
	"home",
	"shell",
}

// GetUserByName returns given user struct by its name.
// It returns an error if we couldn’t fetch the user (does not exist or not connected).
// shadowPasswd is populated only if the shadow database is accessible.
func (c *Cache) GetUserByName(ctx context.Context, username string) (user UserRecord, err error) {
	logger.Debug(ctx, "getting user information from cache for %q", username)

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
	if c.shadowMode > shadowNotAvailableMode {
		query = fmt.Sprintf(queryFmt, ",s.password", ",shadow.shadow s", "AND   p.uid = s.uid")
	}

	row := c.db.QueryRow(query, username)
	u, err := newUserFromScanner(row)
	if err != nil {
		return u, fmt.Errorf(i18n.G("error when getting user %q from cache: %w"), username, err)
	}

	return u, nil
}

// GetAllUserNames returns a list of all user names in the cache.
// It returns an error if we couldn’t fetch the users.
func (c *Cache) GetAllUserNames(ctx context.Context) (users []string, err error) {
	logger.Debug(ctx, "getting all users information from cache")

	rows, err := c.db.Query("SELECT login FROM passwd")
	if err != nil {
		return nil, fmt.Errorf("error when getting all users from cache: %w", err)
	}

	var names []string
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("error when getting all users from cache: %w", err)
		}
		names = append(names, name)
	}

	return names, nil
}

// GetUserByUID returns given user struct by its UID.
// It returns an error if we couldn’t fetch the user (does not exist or not connected).
// shadowPasswd is populated only if the shadow database is accessible.
func (c *Cache) GetUserByUID(ctx context.Context, uid uint) (user UserRecord, err error) {
	logger.Debug(ctx, "getting user information from cache for uid %d", uid)

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
	if c.shadowMode > shadowNotAvailableMode {
		query = fmt.Sprintf(queryFmt, ",s.password", ",shadow.shadow s", "AND   p.uid = s.uid")
	}

	row := c.db.QueryRow(query, uid)
	u, err := newUserFromScanner(row)
	if err != nil {
		return u, fmt.Errorf(i18n.G("error when getting uid %d from cache: %w"), uid, err)
	}

	return u, nil
}

// NextPasswdEntry returns next passwd from the current position within this cache.
// It initializes the passwd query on first run and return ErrNoEnt once done.
func (c *Cache) NextPasswdEntry(ctx context.Context) (u UserRecord, err error) {
	defer func() {
		if err != nil && !errors.Is(err, ErrNoEnt) {
			err = fmt.Errorf(i18n.G("failed to read passwd entry in db: %w"), err)
		}
	}()
	logger.Debug(ctx, "request next passwd entry in db")

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

// ClosePasswdIterator allows to close current iterator underlying request on passwd.
// If none is in process, this is a no-op.
func (c *Cache) ClosePasswdIterator(ctx context.Context) error {
	logger.Debug(ctx, "request to close passwd iteration in db")
	if c.cursorPasswd == nil {
		return nil
	}

	if err := c.cursorPasswd.Close(); err != nil {
		c.cursorPasswd = nil
		return fmt.Errorf(i18n.G("failed to close passwd iterator in db: %w"), err)
	}
	c.cursorPasswd = nil
	return nil
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

// uidOrGidExists check if uid in passwd or gid in groups does exists.
func uidOrGidExists(db *sql.DB, id uint32, username string) (bool, error) {
	row := db.QueryRow("SELECT login,'',-1,-1,-1,-1,-1,-1,-1 from passwd where uid = ? UNION SELECT name,'',-1,-1,-1,-1,-1,-1,-1 from groups where gid = ?", id, id)

	u, err := newUserFromScanner(row)
	if errors.Is(err, ErrNoEnt) {
		return false, nil
	} else if err != nil {
		return true, fmt.Errorf(i18n.G("failed to verify that %d is unique: %w"), id, err)
	}

	// We found one entry, check db inconsistency
	if u.Name == username {
		return true, fmt.Errorf(i18n.G("user already exists in cache"))
	}

	return true, nil
}
