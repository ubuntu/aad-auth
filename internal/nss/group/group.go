package group

import (
	"context"
	"errors"
	"fmt"

	"github.com/ubuntu/aad-auth/internal/cache"
	"github.com/ubuntu/aad-auth/internal/logger"
	"github.com/ubuntu/aad-auth/internal/nss"
)

// Group is the nss group object.
type Group struct {
	name    string   /* username */
	passwd  string   /* user password */
	gid     uint     /* group ID */
	members []string /* Members of the group */
}

var testopts = []cache.Option{}

// NewByName returns a passwd entry from a name.
func NewByName(ctx context.Context, name string) (g Group, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to get group entry from name %q: %w", name, err)
		}
	}()

	logger.Debug(ctx, "Requesting a group entry matching name %q", name)

	if name == "shadow" {
		logger.Debug(ctx, "Ignoring shadow group as it's not in our database")
		return Group{}, nss.ConvertErr(err)
	}

	c, err := cache.New(ctx, testopts...)
	if err != nil {
		return Group{}, nss.ConvertErr(err)
	}
	defer c.Close(ctx)

	grp, err := c.GetGroupByName(ctx, name)
	if err != nil {
		return Group{}, nss.ConvertErr(err)
	}

	return Group{
		name:    grp.Name,
		passwd:  grp.Password,
		gid:     uint(grp.GID),
		members: grp.Members,
	}, nil
}

// NewByGID returns a group entry from a GID.
func NewByGID(ctx context.Context, gid uint) (g Group, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to get group entry from GID %d: %w", gid, err)
		}
	}()

	logger.Debug(ctx, "Requesting an group entry matching GID %d", gid)

	c, err := cache.New(ctx, testopts...)
	if err != nil {

		return Group{}, nss.ConvertErr(err)
	}
	defer c.Close(ctx)

	grp, err := c.GetGroupByGID(ctx, gid)
	if err != nil {
		return Group{}, nss.ConvertErr(err)
	}

	return Group{
		name:    grp.Name,
		passwd:  grp.Password,
		gid:     uint(grp.GID),
		members: grp.Members,
	}, nil
}

var groupIterationCache *cache.Cache

// StartEntryIteration open a new cache for iteration.
// This needs to be called prior to calling NextEntry and be closed with EndEntryIteration.
func StartEntryIteration(ctx context.Context) error {
	if groupIterationCache != nil {
		return nss.ConvertErr(errors.New("group entry iteration already in progress. End it before starting a new one"))
	}

	c, err := cache.New(ctx, testopts...)
	if err != nil {
		return nss.ConvertErr(err)
	}
	groupIterationCache = c
	return nil
}

// EndEntryIteration closes the underlying DB iteration.
func EndEntryIteration(ctx context.Context) error {
	if groupIterationCache == nil {
		logger.Warn(ctx, "group entry iteration ended without initialization first")
		return nil
	}
	c := groupIterationCache
	defer c.Close(ctx)
	groupIterationCache = nil
	return nss.ConvertErr(c.CloseGroupIterator(ctx))
}

// NextEntry returns next available entry in Group. It will returns ENOENT from cache when the iteration is done.
func NextEntry(ctx context.Context) (g Group, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to get group entry: %w", err)
		}
	}()
	logger.Debug(ctx, "get next group entry")

	if groupIterationCache == nil {
		return Group{}, nss.ConvertErr(errors.New("group entry iteration called without initialization first"))
	}

	grp, err := groupIterationCache.NextGroupEntry(ctx)
	if err != nil {
		return Group{}, nss.ConvertErr(err)
	}

	return Group{
		name:    grp.Name,
		passwd:  grp.Password,
		gid:     uint(grp.GID),
		members: grp.Members,
	}, nil
}
