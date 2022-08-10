package cache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/ubuntu/aad-auth/internal/logger"
)

// GroupRecord  returns a group record from the cache.
type GroupRecord struct {
	Name     string
	GID      int64
	Password string
	Members  []string
}

// GetGroupByName returns given group struct by its name.
// It returns an error if we couldn’t fetch the group (does not exist or not connected).
func (c *Cache) GetGroupByName(ctx context.Context, groupname string) (group GroupRecord, err error) {
	logger.Debug(ctx, "getting group information from cache for %q", groupname)

	// Nested query to avoid the case where the user is not found,
	// then all the values are NULL due to the call to GROUP_CONCAT
	query := `
	SELECT * FROM (
		SELECT g.name, g.password, g.gid, group_concat(p.login, ',') as members
		FROM groups g, uid_gid u, passwd p
		WHERE g.name = ?
		AND u.gid = g.gid
		AND p.uid = u.uid
	) WHERE name IS NOT NULL`

	row := c.db.QueryRow(query, groupname)
	g, err := newGroupFromScanner(row)
	if err != nil {
		return g, fmt.Errorf("error when getting group %q from cache: %w", groupname, err)
	}

	return g, nil
}

// GetGroupByGID returns given group struct by its GID.
// It returns an error if we couldn’t fetch the group (does not exist or not connected).
func (c *Cache) GetGroupByGID(ctx context.Context, gid uint) (group GroupRecord, err error) {
	logger.Debug(ctx, "getting group information from cache for gid %d", gid)

	// Nested query to avoid the case where the user is not found,
	// then all the values are NULL due to the call to GROUP_CONCAT
	query := `
	SELECT * FROM (
		SELECT g.name, g.password, g.gid, group_concat(p.login, ',') as members
		FROM groups g, uid_gid u, passwd p
		WHERE g.gid = ?
		AND u.gid = g.gid
		AND p.uid = u.uid
	) WHERE name IS NOT NULL`

	row := c.db.QueryRow(query, gid)
	g, err := newGroupFromScanner(row)
	if err != nil {
		return g, fmt.Errorf("error when getting gid %d from cache: %w", gid, err)
	}

	return g, nil
}

// NextGroupEntry returns next group from the current position within this cache.
// It initializes the group query on first run and return ErrNoEnt once done.
func (c *Cache) NextGroupEntry(ctx context.Context) (g GroupRecord, err error) {
	defer func() {
		if err != nil && !errors.Is(err, ErrNoEnt) {
			err = fmt.Errorf("failed to read group entry in db: %w", err)
		}
	}()
	logger.Debug(ctx, "request next group entry in db")

	if c.cursorGroup == nil {
		query := `
		SELECT * FROM (
			SELECT g.name, g.password, g.gid, group_concat(p.login, ',') as members
			FROM groups g, uid_gid u, passwd p
			WHERE u.gid = g.gid
			AND p.uid = u.uid
			GROUP BY g.name
		) WHERE name IS NOT NULL
		ORDER BY name
		`

		c.cursorGroup, err = c.db.Query(query)
		if err != nil {
			return g, err
		}
	}
	if !c.cursorGroup.Next() {
		if err := c.cursorGroup.Close(); err != nil {
			return g, err
		}
		c.cursorGroup = nil
		return g, ErrNoEnt
	}

	return newGroupFromScanner(c.cursorGroup)
}

// CloseGroupIterator allows to close current iterator underlying request group.
// If none is in process, this is a no-op.
func (c *Cache) CloseGroupIterator(ctx context.Context) error {
	logger.Debug(ctx, "request to close group iteration in db")
	if c.cursorGroup == nil {
		return nil
	}

	if err := c.cursorGroup.Close(); err != nil {
		c.cursorGroup = nil
		return fmt.Errorf("failed to close group iterator in db: %w", err)
	}
	c.cursorGroup = nil
	return nil
}

// newGroupFromScanner abstracts the row request deserialization to GroupRecord.
// It returns ErrNoEnt in case of no element found.
func newGroupFromScanner(r rowScanner) (g GroupRecord, err error) {
	var members string
	if err := r.Scan(&g.Name, &g.Password, &g.GID, &members); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = ErrNoEnt
		}
		return GroupRecord{}, err
	}
	g.Members = strings.Split(members, ",")

	return g, nil
}
