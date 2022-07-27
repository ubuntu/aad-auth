package cache_test

import (
	"context"
	"os/user"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/cache"
)

func newCacheForTests(t *testing.T, cacheDir string, closeWithoutDelay, withoutCleanup bool) (c *cache.Cache) {
	t.Helper()

	uid, gid := getCurrentUidGid(t)
	opts := append([]cache.Option{}, cache.WithCacheDir(cacheDir),
		cache.WithRootUID(uid), cache.WithRootGID(gid), cache.WithShadowGID(gid))

	if closeWithoutDelay {
		opts = append(opts, cache.WithTeardownDuration(0))
	}
	if withoutCleanup {
		opts = append(opts, cache.WithOfflineCredentialsExpiration(0))
	}

	c, err := cache.New(context.Background(), opts...)
	require.NoError(t, err, "Setup: should be able to create a cache")
	t.Cleanup(func() { c.Close(context.Background()) })

	return c
}

func getCurrentUidGid(t *testing.T) (int, int) {
	t.Helper()

	u, err := user.Current()
	require.NoError(t, err, "Setup: could not get current user")

	uid, err := strconv.Atoi(u.Uid)
	require.NoError(t, err, "Setup: could not convert current uid")
	gid, err := strconv.Atoi(u.Gid)
	require.NoError(t, err, "Setup: could not convert current gid")

	return uid, gid
}
