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
	//cache.WithCacheDir("../cache"), cache.WithRootUid(1000), cache.WithRootGid(1000), cache.WithShadowGid(1000),
}

// NewByName returns a passwd entry from a name.
func NewByName(name string) (g Group, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to get group entry from name %q: %w", name, err)
		}
	}()

	ctx := context.Background()
	pam.LogDebug(context.Background(), "Requesting a group entry matching name %q", name)

	c, err := cache.New(ctx, testopts...)
	if err != nil {
		return Group{}, nss.ErrUnavailableENoEnt
	}
	defer c.Close()

	grp, err := c.GetGroupByName(ctx, name)
	if err != nil {
		// TODO: remove this wrapper and just print logs on error before converting to known format for the C lib.
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
func NewByGID(gid uint) (g Group, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to get group entry from GID %d: %w", gid, err)
		}
	}()

	ctx := context.Background()
	pam.LogDebug(context.Background(), "Requesting an group entry matching GID %d", gid)

	c, err := cache.New(ctx, testopts...)
	if err != nil {

		return Group{}, nss.ErrUnavailableENoEnt
	}
	defer c.Close()

	grp, err := c.GetGroupByGid(ctx, gid)
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

var cacheIterateEntries *cache.Cache

// NextEntry returns next available entry in Group. It will returns ENOENT from cache when the iteration is done.
// It automatically opens and close the cache on first/last iteration.
func NextEntry() (g Group, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to get group entry: %w", err)
		}
	}()
	pam.LogDebug(context.Background(), "get next group entry")

	if cacheIterateEntries == nil {
		cacheIterateEntries, err = cache.New(context.Background(), testopts...)
		if err != nil {
			return Group{}, nss.ErrUnavailableENoEnt
		}
	}

	grp, err := cacheIterateEntries.NextGroupEntry()
	if errors.Is(err, cache.ErrNoEnt) {
		_ = cacheIterateEntries.Close()
		cacheIterateEntries = nil
		return Group{}, nss.ConvertErr(err)
	} else if err != nil {
		return Group{}, nss.ConvertErr(err)
	}

	return Group{
		name:    grp.Name,
		passwd:  grp.Password,
		gid:     uint(grp.GID),
		members: grp.Members,
	}, nil
}
