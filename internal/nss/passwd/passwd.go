package passwd

import (
	"context"
	"errors"
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

var passwdIterationCache *cache.Cache

// StartEntryIteration open a new cache for iteration.
// This needs to be called prior to calling NextEntry and be closed with EndEntryIteration.
func StartEntryIteration(ctx context.Context) error {
	if passwdIterationCache != nil {
		return nss.ConvertErr(errors.New("passwd entry iteration already in progress. End it before starting a new one"))
	}

	c, err := cache.New(ctx, testopts...)
	if err != nil {
		return nss.ConvertErr(err)
	}
	passwdIterationCache = c
	return nil
}

// EndEntryIteration closes the underlying DB iterator.
func EndEntryIteration(ctx context.Context) error {
	if passwdIterationCache == nil {
		logger.Warn(ctx, "passwd entry iteration ended without initialization first")
		return nil
	}
	c := passwdIterationCache
	defer c.Close(ctx)
	passwdIterationCache = nil
	return nss.ConvertErr(c.ClosePasswdIterator(ctx))
}

// NextEntry returns next available entry in Passwd. It will returns ENOENT from cache when the iteration is done.
// You need to open StartEntryIteration prior to get to any Entry.
func NextEntry(ctx context.Context) (p Passwd, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to get passwd entry: %w", err)
		}
	}()
	logger.Debug(ctx, "get next passwd entry")

	if passwdIterationCache == nil {
		return Passwd{}, nss.ConvertErr(errors.New("passwd entry iteration called without initialization first"))
	}

	u, err := passwdIterationCache.NextPasswdEntry(ctx)
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
