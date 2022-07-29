package cache_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/cache"
	"github.com/ubuntu/aad-auth/internal/testutils"
)

// newCacheForTests returns a cache that is closed automatically, with permissions set to current user.
func newCacheForTests(t *testing.T, cacheDir string, closeWithoutDelay, withoutCleanup bool) (c *cache.Cache) {
	t.Helper()

	uid, gid := testutils.GetCurrentUidGid(t)
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

type userInfos struct {
	name     string
	uid      int
	password string
}

var (
	usersForTests = map[string]userInfos{
		"myuser@domain.com":    {"myuser@domain.com", 1929326240, "my password"},
		"otheruser@domain.com": {"otheruser@domain.com", 165119648, "other password"},
		"user@otherdomain.com": {"user@otherdomain.com", 165119649, "other user domain password"},
	}
	usersForTestsByUid = make(map[uint]userInfos)
)

func init() {
	// populate usersForTestByUid
	for _, info := range usersForTests {
		usersForTestsByUid[uint(info.uid)] = info
	}
}

// insertUsersInDb inserts usersForTests after opening a cache at cacheDir.
func insertUsersInDb(t *testing.T, cacheDir string) {
	t.Helper()

	c := newCacheForTests(t, cacheDir, true, false)
	defer c.Close(context.Background())
	for u, info := range usersForTests {
		err := c.Update(context.Background(), u, info.password)
		require.NoError(t, err, "Setup: can’t insert user %v to db", u)
	}
}
