package cache_test

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/cache"
	"github.com/ubuntu/aad-auth/internal/testutils"
)

// newCacheForTests returns a cache that is closed automatically, with permissions set to current user.
func newCacheForTests(t *testing.T, cacheDir string, closeWithoutDelay, withoutCleanup bool) (c *cache.Cache) {
	t.Helper()

	uid, gid := testutils.GetCurrentUIDGID(t)
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
	uid      int64
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

	// The randomness in map iterating was causing problems with the tests.
	// Some test users were getting different IDs based on the order they were
	// inserted in the cache. To fix that, the test users will be inserted in
	// ASCII order.
	keys := getSortedKeys(usersForTests)

	for _, k := range keys {
		u := usersForTests[k]
		err := c.Update(context.Background(), u.name, u.password, "/home/%f", "/bin/bash")
		require.NoError(t, err, "Setup: can't insert user %v to db", u.name)
	}
}

func getSortedKeys(usersMap map[string]userInfos) []string {
	keys := make([]string, 0, len(usersForTests))
	for k := range usersMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
