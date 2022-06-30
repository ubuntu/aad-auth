package group

import (
	"context"
	"errors"
	"fmt"

	"github.com/ubuntu/aad-auth/internal/cache"
	"github.com/ubuntu/aad-auth/internal/nss"
	"github.com/ubuntu/aad-auth/internal/pam"
)

type Group struct {
	name    string   /* username */
	passwd  string   /* user password */
	gid     uint     /* group ID */
	members []string /* Members of the group */
}

var testopts = []cache.Option{
	cache.WithCacheDir("../cache"), cache.WithRootUid(1000), cache.WithRootGid(1000), cache.WithShadowGid(1000),
}

// NewByName returns a passwd entry from a name.
func NewByName(name string) (g Group, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to get entry from name %q: %v", name, err)
		}
	}()

	ctx := context.Background()
	pam.LogDebug(context.Background(), "Requesting an entry matching name %q", name)

	c, err := cache.New(ctx, testopts...)
	if err != nil {
		return Group{}, nss.ErrUnavailable
	}
	defer c.Close()

	group, err := c.GetGroupByName(ctx, name)
	if err != nil {
		// TODO: remove this wrapper and just print logs on error before converting to known format for the C lib.
		return Group{}, nss.ErrNoEntriesToNotFound(err)
	}

	return Group{
		name:    group.Name,
		passwd:  group.Password,
		gid:     uint(group.GID),
		members: group.Members,
	}, nil
}

// NewByGID returns a group entry from a GID.
func NewByGID(gid uint) (g Group, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to get entry from GID %d: %v", gid, err)
		}
	}()

	ctx := context.Background()
	pam.LogDebug(context.Background(), "Requesting an entry matching GID %d", gid)

	c, err := cache.New(ctx, testopts...)
	if err != nil {

		return Group{}, nss.ErrUnavailable
	}
	defer c.Close()

	group, err := c.GetGroupByGid(ctx, gid)
	if err != nil {
		return Group{}, nss.ErrNoEntriesToNotFound(err)
	}

	return Group{
		name:    group.Name,
		passwd:  group.Password,
		gid:     uint(group.GID),
		members: group.Members,
	}, nil
}

var cacheIterateEntries *cache.Cache

// NextEntry returns next available entry in Group. It will returns ENOENT from cache when the iteration is done.
// It automatically opens and close the cache on first/last iteration.
func NextEntry() (g Group, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to get group entry: %v", err)
		}
	}()
	pam.LogDebug(context.Background(), "get next group entry")

	if cacheIterateEntries == nil {
		cacheIterateEntries, err = cache.New(context.Background(), testopts...)
		if err != nil {
			return Group{}, err
		}
	}

	grp, err := cacheIterateEntries.NextGroupEntry()
	if errors.Is(err, cache.ErrNoEnt) {
		_ = cacheIterateEntries.Close()
		cacheIterateEntries = nil
		return Group{}, err
	} else if err != nil {
		return Group{}, err
	}

	return Group{
		name:    grp.Name,
		passwd:  "x",
		gid:     uint(grp.GID),
		members: grp.Members,
	}, nil
}
