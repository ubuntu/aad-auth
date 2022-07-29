package passwd

import (
	"context"
	"fmt"

	"github.com/ubuntu/aad-auth/internal/cache"
	"github.com/ubuntu/aad-auth/internal/logger"
	"github.com/ubuntu/aad-auth/internal/nss"
)

// Passwd is the nss passwd object.
type Passwd struct {
	name   string /* username */
	passwd string /* user password */
	uid    uint   /* user ID */
	gid    uint   /* group ID */
	gecos  string /* user information */
	dir    string /* home directory */
	shell  string /* shell program */
}

var testopts = []cache.Option{}

// NewByName returns a passwd entry from a name.
func NewByName(ctx context.Context, name string) (p Passwd, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to get passwd entry from name %q: %w", name, err)
		}
	}()

	logger.Debug(ctx, "Requesting a passwd entry matching name %q", name)

	c, err := cache.New(ctx, testopts...)
	if err != nil {
		return Passwd{}, nss.ConvertErr(err)
	}
	defer c.Close(ctx)

	u, err := c.GetUserByName(ctx, name)
	if err != nil {
		return Passwd{}, nss.ConvertErr(err)
	}

	return Passwd{
		name:   u.Name,
		passwd: u.Passwd,
		uid:    uint(u.UID),
		gid:    uint(u.GID),
		gecos:  u.Gecos,
		dir:    u.Home,
		shell:  u.Shell,
	}, nil
}

// NewByUID returns a passwd entry from an UID.
func NewByUID(ctx context.Context, uid uint) (p Passwd, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to get passwd entry from UID %d: %w", uid, err)
		}
	}()

	logger.Debug(ctx, "Requesting a passwd entry matching UID %d", uid)

	c, err := cache.New(ctx, testopts...)
	if err != nil {
		return Passwd{}, nss.ConvertErr(err)
	}
	defer c.Close(ctx)

	u, err := c.GetUserByUID(ctx, uid)
	if err != nil {
		return Passwd{}, nss.ConvertErr(err)
	}

	return Passwd{
		name:   u.Name,
		passwd: u.Passwd,
		uid:    uint(u.UID),
		gid:    uint(u.GID),
		gecos:  u.Gecos,
		dir:    u.Home,
		shell:  u.Shell,
	}, nil
}

// StartEntryIteration open a new cache for iteration.
func StartEntryIteration(ctx context.Context) error {
	c, err := cache.New(ctx, testopts...)
	if err != nil {
		return nss.ConvertErr(err)
	}
	defer c.Close(ctx)
	return nss.ConvertErr(c.ClosePasswdIterator(ctx))
}

// EndEntryIteration closes the underlying DB iterator.
func EndEntryIteration(ctx context.Context) error {
	c, err := cache.New(ctx, testopts...)
	if err != nil {
		return nss.ConvertErr(err)
	}
	defer c.Close(ctx)
	return nss.ConvertErr(c.ClosePasswdIterator(ctx))
}

// NextEntry returns next available entry in Passwd. It will returns ENOENT from cache when the iteration is done.
// It automatically opens and close the cache on first/last iteration.
func NextEntry(ctx context.Context) (p Passwd, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to get passwd entry: %w", err)
		}
	}()
	logger.Debug(ctx, "get next passwd entry")

	c, err := cache.New(ctx, testopts...)
	if err != nil {
		return Passwd{}, nss.ConvertErr(err)
	}
	defer c.Close(ctx)

	u, err := c.NextPasswdEntry(ctx)
	if err != nil {
		return Passwd{}, nss.ConvertErr(err)
	}

	return Passwd{
		name:   u.Name,
		passwd: u.Passwd,
		uid:    uint(u.UID),
		gid:    uint(u.GID),
		gecos:  u.Gecos,
		dir:    u.Home,
		shell:  u.Shell,
	}, nil
}
