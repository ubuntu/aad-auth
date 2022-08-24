package cache_test

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/cache"
)

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
	usersForTestsByUID = make(map[uint]userInfos)
)

func init() {
	// populate usersForTestByUid
	for _, info := range usersForTests {
		usersForTestsByUID[uint(info.uid)] = info
	}
}

// insertUsersInDb inserts usersForTests after opening a cache at cacheDir.
func insertUsersInDb(t *testing.T, cacheDir string) {
	t.Helper()

	c := cache.NewCacheForTests(t, cacheDir, cache.WithTeardownDuration(0))
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
