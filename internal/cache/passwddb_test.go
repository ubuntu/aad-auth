package cache_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/cache"
	"github.com/ubuntu/aad-auth/internal/testutils"
	"golang.org/x/crypto/bcrypt"
)

func TestGetUserByName(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		name       string
		shadowMode int

		wantErr bool
	}{
		"get existing user by name with encrypted password": {name: "myuser@domain.com", shadowMode: cache.ShadowROMode},
		"have access to encrypted password in RW too":       {name: "myuser@domain.com", shadowMode: cache.ShadowRWMode},
		"no encrypted password":                             {name: "myuser@domain.com", shadowMode: cache.ShadowNotAvailableMode},

		// error cases
		"error on non existing user": {name: "notexist@domain.com", wantErr: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cacheDir := t.TempDir()
			testutils.PrepareDBsForTests(t, cacheDir, "users_in_db", cache.WithShadowMode(tc.shadowMode))

			c := testutils.NewCacheForTests(t, cacheDir, cache.WithShadowMode(tc.shadowMode))

			u, err := c.GetUserByName(context.Background(), tc.name)
			if tc.wantErr {
				require.Error(t, err, "GetUserByName should have returned an error and hasn’t")
				assert.ErrorIs(t, err, cache.ErrNoEnt, "Known error returned should be of type ErrNoEnt")
				return
			}
			require.NoError(t, err, "GetUserByName should not have returned an error and has")

			// Handle dynamic fields
			// Checks if the lastOnlineAuth value was loaded properly.
			assert.False(t, u.LastOnlineAuth.IsZero(), "Last Online should not be zero.")
			u.LastOnlineAuth = time.Unix(0, 0)

			// Validate password
			if tc.shadowMode > 0 {
				err := bcrypt.CompareHashAndPassword([]byte(u.ShadowPasswd), []byte(usersForTests[tc.name].password))
				assert.NoError(t, err, "Encrypted passwords should match the insertion")
				u.ShadowPasswd = ""
			}

			wantUser := cache.UserRecord{
				Name:           tc.name,
				Passwd:         "x",
				UID:            usersForTests[tc.name].uid,
				GID:            usersForTests[tc.name].uid, // GID match UID
				Gecos:          usersForTests[tc.name].gecos,
				Home:           filepath.Join("/home", tc.name), // Default (fallback) home
				Shell:          "/bin/bash",                     // Default (fallback) home
				ShadowPasswd:   "",                              // already hanlded
				LastOnlineAuth: time.Unix(0, 0),                 // we will match it manually
			}

			assert.Equal(t, wantUser, u, "User should match input")
		})
	}
}

func TestGetUserByUID(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		uid        uint
		shadowMode int

		wantErr bool
	}{
		"get existing user by uid with encrypted password": {uid: 1929326240, shadowMode: cache.ShadowROMode},
		"have access to encrypted password in RW too":      {uid: 1929326240, shadowMode: cache.ShadowRWMode},
		"no encrypted password":                            {uid: 1929326240, shadowMode: cache.ShadowNotAvailableMode},

		// error cases
		"error on non existing user": {uid: 4242, wantErr: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cacheDir := t.TempDir()
			testutils.PrepareDBsForTests(t, cacheDir, "users_in_db", cache.WithShadowMode(tc.shadowMode))

			c := testutils.NewCacheForTests(t, cacheDir, cache.WithShadowMode(tc.shadowMode))

			u, err := c.GetUserByUID(context.Background(), tc.uid)
			if tc.wantErr {
				require.Error(t, err, "GetUserByName should have returned an error and hasn’t")
				assert.ErrorIs(t, err, cache.ErrNoEnt, "Known error returned should be of type ErrNoEnt")
				return
			}
			require.NoError(t, err, "GetUserByName should not have returned an error and has")

			// Handle dynamic fields
			// Checks if the lastOnlineAuth value was loaded properly.
			assert.False(t, u.LastOnlineAuth.IsZero(), "Last Online should not be zero.")
			u.LastOnlineAuth = time.Unix(0, 0)

			// Validate password
			if tc.shadowMode > 0 {
				err := bcrypt.CompareHashAndPassword([]byte(u.ShadowPasswd), []byte(usersForTestsByUID[tc.uid].password))
				assert.NoError(t, err, "Encrypted passwords should match the insertion")
				u.ShadowPasswd = ""
			}

			wantUser := cache.UserRecord{
				Name:           usersForTestsByUID[tc.uid].name,
				Passwd:         "x",
				UID:            int64(tc.uid),
				GID:            int64(tc.uid), // GID match UID
				Gecos:          usersForTestsByUID[tc.uid].gecos,
				Home:           filepath.Join("/home", usersForTestsByUID[tc.uid].name), // Default (fallback) home
				Shell:          "/bin/bash",                                             // Default (fallback) home
				ShadowPasswd:   "",                                                      // already hanlded
				LastOnlineAuth: time.Unix(0, 0),                                         // we will match it manually
			}

			assert.Equal(t, wantUser, u, "User should match input")
		})
	}
}

func TestNextPasswdEntry(t *testing.T) {
	t.Parallel()

	// We iterate over all entries in the DB to ensure we have listed them all
	wanted := make(map[string]cache.UserRecord)

	for n, info := range usersForTests {
		wanted[n] = cache.UserRecord{
			Name:           n,
			Passwd:         "x",
			UID:            info.uid,
			GID:            info.uid, // GID match UID
			Gecos:          info.gecos,
			Home:           filepath.Join("/home", n), // Default (fallback) home
			Shell:          "/bin/bash",               // Default (fallback) home
			ShadowPasswd:   "",                        // we don’t have access to shadow password in this mode.
			LastOnlineAuth: time.Unix(0, 0),           // we will match it manually
		}
	}

	cacheDir := t.TempDir()
	testutils.PrepareDBsForTests(t, cacheDir, "users_in_db")

	c := testutils.NewCacheForTests(t, cacheDir)

	// Iterate over all entries
	numIteration := len(wanted)
	for i := 0; i < numIteration; i++ {
		u, err := c.NextPasswdEntry(context.Background())
		require.NoError(t, err, "NextPasswdEntry should initiate and returns values without any error")

		// Checks if the lastOnlineAuth value was loaded properly.
		assert.False(t, u.LastOnlineAuth.IsZero(), "Last Online should not be zero.")
		u.LastOnlineAuth = time.Unix(0, 0)

		wantUser, found := wanted[u.Name]
		require.True(t, found, "%v should be in %v", u.Name, wanted)
		assert.Equal(t, wantUser, u, "User should match what we inserted")
	}

	// Final iteration: should return ENoEnt to ends it
	u, err := c.NextPasswdEntry(context.Background())
	require.ErrorIs(t, err, cache.ErrNoEnt, "final iteration should return ENOENT, but we got %v", u)
}

func TestNextPasswdEntryNoUser(t *testing.T) {
	t.Parallel()

	c := testutils.NewCacheForTests(t, t.TempDir())
	u, err := c.NextPasswdEntry(context.Background())
	require.ErrorIs(t, err, cache.ErrNoEnt, "first and final iteration should return ENOENT, but we got %v", u)
}

func TestNextPasswdCloseBeforeIterationEnds(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()
	testutils.PrepareDBsForTests(t, cacheDir, "users_in_db")

	c := testutils.NewCacheForTests(t, cacheDir)

	_, err := c.NextPasswdEntry(context.Background())
	require.NoError(t, err, "NextPasswdEntry should initiate and returns values without any error")

	// This closes underlying iterator
	err = c.ClosePasswdIterator(context.Background())
	require.NoError(t, err, "No error should occur when closing the iterator in tests")

	// Trying to iterate for all entries
	numIteration := len(usersForTests)
	for i := 0; i < numIteration; i++ {
		_, err := c.NextPasswdEntry(context.Background())
		require.NoError(t, err, "NextPasswdEntry should initiate and returns values without any error")
	}

	// Final iteration: should return ENoEnt to ends it
	u, err := c.NextPasswdEntry(context.Background())
	require.ErrorIs(t, err, cache.ErrNoEnt, "final iteration should return ENOENT, but we got %v", u)

	c.Close(context.Background())
	c.WaitForCacheClosed()
}
