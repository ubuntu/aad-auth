package cache_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/cache"
	"github.com/ubuntu/aad-auth/internal/testutils"
	"golang.org/x/crypto/bcrypt"
)

func TestGetShadowByName(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		name       string
		shadowMode int

		wantErr       bool
		wantErrENOENT bool
	}{
		"get existing shadow information for user by name with encrypted password": {name: "myuser@domain.com", shadowMode: cache.ShadowROMode},
		"have access to encrypted password in RW too":                              {name: "myuser@domain.com", shadowMode: cache.ShadowRWMode},

		// error cases
		"error on non existing user shadow": {name: "notexist@domain.com", shadowMode: cache.ShadowROMode, wantErr: true, wantErrENOENT: true},
		"error on no access to shadow file": {name: "myuser@domain.com", shadowMode: cache.ShadowNotAvailableMode, wantErr: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cacheDir := t.TempDir()
			testutils.PrepareDBsForTests(t, cacheDir, "users_in_db", cache.WithShadowMode(tc.shadowMode))

			c := testutils.NewCacheForTests(t, cacheDir, cache.WithShadowMode(tc.shadowMode))

			s, err := c.GetShadowByName(context.Background(), tc.name)
			if tc.wantErr {
				require.Error(t, err, "GetShadowByName should have returned an error and hasn’t")
				if tc.wantErrENOENT {
					assert.ErrorIs(t, err, cache.ErrNoEnt, "Known error returned should be of type ErrNoEnt")
				}
				return
			}
			require.NoError(t, err, "GetShadowByName should not have returned an error and has")

			// Validate password (dynamic field)
			err = bcrypt.CompareHashAndPassword([]byte(s.Password), []byte(usersForTests[tc.name].password))
			assert.NoError(t, err, "Encrypted passwords should match the insertion")
			s.Password = ""

			wantUser := cache.ShadowRecord{
				Name:           tc.name,
				Password:       "",
				LastPwdChange:  -1,
				MaxPwdAge:      -1,
				PwdWarnPeriod:  -1,
				PwdInactivity:  -1,
				MinPwdAge:      -1,
				ExpirationDate: -1,
			}

			assert.Equal(t, wantUser, s, "User should match input")
		})
	}
}

func TestNextShadowEntry(t *testing.T) {
	t.Parallel()

	// We iterate over all entries in the DB to ensure we have listed them all
	wanted := make(map[string]cache.ShadowRecord)

	for n := range usersForTests {
		wanted[n] = cache.ShadowRecord{
			Name:           n,
			Password:       "",
			LastPwdChange:  -1,
			MaxPwdAge:      -1,
			PwdWarnPeriod:  -1,
			PwdInactivity:  -1,
			MinPwdAge:      -1,
			ExpirationDate: -1,
		}
	}

	cacheDir := t.TempDir()
	testutils.PrepareDBsForTests(t, cacheDir, "users_in_db")

	c := testutils.NewCacheForTests(t, cacheDir)

	// Iterate over all entries
	numIteration := len(wanted)
	for i := 0; i < numIteration; i++ {
		s, err := c.NextShadowEntry(context.Background())
		require.NoError(t, err, "NextShadowEntry should initiate and returns values without any error")

		wantUser, found := wanted[s.Name]
		require.True(t, found, "%v should be in %v", s.Name, wanted)

		// Validate password (dynamic field)
		err = bcrypt.CompareHashAndPassword([]byte(s.Password), []byte(usersForTests[s.Name].password))
		assert.NoError(t, err, "Encrypted passwords should match the insertion")
		s.Password = ""

		assert.Equal(t, wantUser, s, "Shadow should match the user what we inserted")
	}

	// Final iteration: should return ENoEnt to ends it
	u, err := c.NextShadowEntry(context.Background())
	require.ErrorIs(t, err, cache.ErrNoEnt, "final iteration should return ENOENT, but we got %v", u)
}

func TestNextShadowEntryNoShadow(t *testing.T) {
	t.Parallel()

	c := testutils.NewCacheForTests(t, t.TempDir())
	s, err := c.NextShadowEntry(context.Background())
	require.ErrorIs(t, err, cache.ErrNoEnt, "first and final iteration should return ENOENT, but we got %v", s)
}

func TestNextShadowCloseBeforeIterationEnds(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()
	testutils.PrepareDBsForTests(t, cacheDir, "users_in_db")

	c := testutils.NewCacheForTests(t, cacheDir)

	_, err := c.NextShadowEntry(context.Background())
	require.NoError(t, err, "NextShadowEntry should initiate and returns values without any error")

	// This closes underlying iterator
	err = c.CloseShadowIterator(context.Background())
	require.NoError(t, err, "No error should occur when closing the iterator in tests")

	// Trying to iterate for all entries
	numIteration := len(usersForTests)
	for i := 0; i < numIteration; i++ {
		_, err := c.NextShadowEntry(context.Background())
		require.NoError(t, err, "NextShadowEntry should initiate and returns values without any error")
	}

	// Final iteration: should return ENoEnt to ends it
	s, err := c.NextShadowEntry(context.Background())
	require.ErrorIs(t, err, cache.ErrNoEnt, "final iteration should return ENOENT, but we got %v", s)

	c.Close(context.Background())
	c.WaitForCacheClosed()
}
