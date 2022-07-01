package cache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ubuntu/aad-auth/internal/pam"
)

// ShadowRecord returns a shadow record from the cache
type ShadowRecord struct {
	Name           string
	Password       string
	LastPwdChange  int
	MaxPwdAge      int
	PwdWarnPeriod  int
	PwdInactivity  int
	MinPwdAge      int
	ExpirationDate int
}

// GetShadowByName returns given shadow struct by its name.
// It returns an error if we couldn’t fetch the shadow entry (does not exist or not connected).
func (c *Cache) GetShadowByName(ctx context.Context, username string) (swr ShadowRecord, err error) {
	pam.LogDebug(ctx, "getting shadow information from cache for %q", username)

	if !c.hasShadow {
		return swr, errors.New("need shadow to be accessible to query on it")
	}

	query := `
	SELECT p.login, s.password, s.last_pwd_change, s.min_pwd_age, s.max_pwd_age, s.pwd_warn_period, s.pwd_inactivity, s.expiration_date
	FROM passwd p, shadow.shadow s
	WHERE p.uid = s.uid
	AND p.login = ?
	`
	row := c.db.QueryRow(query, username)
	swr, err = newShadowFromScanner(row)
	if err != nil {
		return swr, fmt.Errorf("error when getting shadow matching %q from cache: %w", username, err)
	}

	return swr, nil
}

// NextShadowEntry returns next shadow from the current position within this cache.
// It initializes the shadow query on first run and return ErrNoEnt once done.
func (c *Cache) NextShadowEntry() (swr ShadowRecord, err error) {
	defer func() {
		if err != nil && !errors.Is(err, ErrNoEnt) {
			err = fmt.Errorf("failed to read shadow entry in db: %v", err)
		}
	}()
	pam.LogDebug(context.Background(), "request next shadow entry in db")

	if c.cursorShadow == nil {
		query := `
		SELECT p.login, s.password, s.last_pwd_change, s.min_pwd_age, s.max_pwd_age, s.pwd_warn_period, s.pwd_inactivity, s.expiration_date
		FROM passwd p, shadow.shadow s
		WHERE p.uid = s.uid
		`

		c.cursorShadow, err = c.db.Query(query)
		if err != nil {
			return swr, err
		}
	}
	if !c.cursorShadow.Next() {
		if err := c.cursorShadow.Close(); err != nil {
			return swr, err
		}
		c.cursorShadow = nil
		return swr, ErrNoEnt
	}

	return newShadowFromScanner(c.cursorShadow)
}

// newShadowFromScanner abstracts the row request deserialization to ShadowRecord.
// It returns ErrNoEnt in case of no element found.
func newShadowFromScanner(r rowScanner) (swr ShadowRecord, err error) {
	if err := r.Scan(&swr.Name, &swr.Password, &swr.LastPwdChange, &swr.MinPwdAge, &swr.MaxPwdAge, &swr.PwdWarnPeriod, &swr.PwdInactivity, &swr.ExpirationDate); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = ErrNoEnt
		}
		return ShadowRecord{}, err
	}

	return swr, nil
}
