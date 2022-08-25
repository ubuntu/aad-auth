package cache_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/cache"
	"github.com/ubuntu/aad-auth/internal/testutils"
)

func TestGetGroupByName(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		name string

		wantErr bool
	}{
		"get existing group by name": {name: "myuser@domain.com"},

		// error cases
		"error on non existing group": {name: "notexist@domain.com", wantErr: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cacheDir := t.TempDir()
			testutils.PrepareDBsForTests(t, cacheDir, "users_in_db")
			c := testutils.NewCacheForTests(t, cacheDir)

			g, err := c.GetGroupByName(context.Background(), tc.name)
			if tc.wantErr {
				require.Error(t, err, "GetGroupByName should have returned an error and hasn’t")
				assert.ErrorIs(t, err, cache.ErrNoEnt, "Known error returned should be of type ErrNoEnt")
				return
			}
			require.NoError(t, err, "GetGroupByName should not have returned an error and has")

			wantGroup := cache.GroupRecord{
				Name:     tc.name,
				GID:      usersForTests[tc.name].uid, // GID match user with same name GID.
				Password: "x",
				Members:  []string{tc.name}, // there is one member, which is the user with the same name.
			}

			assert.Equal(t, wantGroup, g, "Group should match input")
		})
	}
}

func TestGetGroupByGID(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		gid uint

		wantErr bool
	}{
		"get existing group by gid": {gid: 1929326240},

		// error cases
		"error on non existing group": {gid: 4242, wantErr: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cacheDir := t.TempDir()
			testutils.PrepareDBsForTests(t, cacheDir, "users_in_db")

			c := testutils.NewCacheForTests(t, cacheDir)

			g, err := c.GetGroupByGID(context.Background(), tc.gid)
			if tc.wantErr {
				require.Error(t, err, "GetGroupByGID should have returned an error and hasn’t")
				assert.ErrorIs(t, err, cache.ErrNoEnt, "Known error returned should be of type ErrNoEnt")
				return
			}
			require.NoError(t, err, "GetGroupByGID should not have returned an error and has")

			wantGroup := cache.GroupRecord{
				Name:     usersForTestsByUID[tc.gid].name, // Name match user with same name UID/GID.
				GID:      int64(tc.gid),
				Password: "x",
				Members:  []string{usersForTestsByUID[tc.gid].name}, // there is one member, which is the user with the same UID/GID..
			}

			assert.Equal(t, wantGroup, g, "Group should match input")
		})
	}
}

func TestNextGroupEntry(t *testing.T) {
	t.Parallel()

	// We iterate over all entries in the DB to ensure we have listed them all
	wanted := make(map[string]cache.GroupRecord)

	for n, info := range usersForTests {
		wanted[n] = cache.GroupRecord{
			Name:     n,        // username is the group name
			GID:      info.uid, // GID match user with same name GID.
			Password: "x",
			Members:  []string{n}, // there is one member, which is the user with the same name.
		}
	}

	cacheDir := t.TempDir()
	testutils.PrepareDBsForTests(t, cacheDir, "users_in_db")

	c := testutils.NewCacheForTests(t, cacheDir)

	// Iterate over all entries
	numIteration := len(wanted)
	for i := 0; i < numIteration; i++ {
		g, err := c.NextGroupEntry(context.Background())
		require.NoError(t, err, "numIteration should initiate and returns values without any error")

		wantGroup, found := wanted[g.Name]
		require.True(t, found, "%v should be in %v", g.Name, wanted)
		assert.Equal(t, wantGroup, g, "Group should match what we inserted")
	}

	// Final iteration: should return ENoEnt to ends it
	g, err := c.NextGroupEntry(context.Background())
	require.ErrorIs(t, err, cache.ErrNoEnt, "final iteration should return ENOENT, but we got %v", g)
}

func TestNextGroupEntryNoGroup(t *testing.T) {
	t.Parallel()

	c := testutils.NewCacheForTests(t, t.TempDir())
	g, err := c.NextGroupEntry(context.Background())
	require.ErrorIs(t, err, cache.ErrNoEnt, "first and final iteration should return ENOENT, but we got %v", g)
}

func TestNextGroupCloseBeforeIterationEnds(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()
	testutils.PrepareDBsForTests(t, cacheDir, "users_in_db")

	c := testutils.NewCacheForTests(t, cacheDir)

	_, err := c.NextGroupEntry(context.Background())
	require.NoError(t, err, "NextGroupEntry should initiate and returns values without any error")

	// This closes underlying iterator
	err = c.CloseGroupIterator(context.Background())
	require.NoError(t, err, "No error should occur when closing the iterator in tests")

	// Trying to iterate for all entries
	numIteration := len(usersForTests)
	for i := 0; i < numIteration; i++ {
		_, err := c.NextGroupEntry(context.Background())
		require.NoError(t, err, "NextGroupEntry should initiate and returns values without any error")
	}

	// Final iteration: should return ENoEnt to ends it
	g, err := c.NextGroupEntry(context.Background())
	require.ErrorIs(t, err, cache.ErrNoEnt, "final iteration should return ENOENT, but we got %v", g)

	c.Close(context.Background())
	c.WaitForCacheClosed()
}
