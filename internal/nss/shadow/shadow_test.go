package shadow_test

import (
	"context"
	"flag"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/cache"
	"github.com/ubuntu/aad-auth/internal/nss"
	"github.com/ubuntu/aad-auth/internal/nss/shadow"
	"github.com/ubuntu/aad-auth/internal/testutils"
)

func TestNewByName(t *testing.T) {
	tests := map[string]struct {
		name         string
		failingCache bool

		wantErrType error
	}{
		"get existing user by name": {name: "myuser@domain.com"},

		// error cases
		"error on non existing user":   {name: "notexists@domain.com", wantErrType: nss.ErrNotFoundENoEnt},
		"error on cache not available": {name: "myuser@domain.com", failingCache: true, wantErrType: nss.ErrUnavailableENoEnt},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			cacheDir := t.TempDir()
			testutils.CopyDBAndFixPermissions(t, "../testdata/users_in_db", cacheDir)

			uid, gid := testutils.GetCurrentUIDGID(t)
			opts := []cache.Option{cache.WithCacheDir(cacheDir), cache.WithRootUID(uid), cache.WithRootGID(gid), cache.WithShadowGID(gid)}
			if tc.failingCache {
				opts = append(opts, cache.WithRootUID(4242))
			}
			shadow.SetCacheOption(opts...)

			got, err := shadow.NewByName(context.Background(), tc.name)
			if tc.wantErrType != nil {
				require.Error(t, err, "NewByName should have returned an error and hasn’t")
				require.ErrorIs(t, err, tc.wantErrType, "NewByName has not returned expected error type")
				return
			}
			require.NoError(t, err, "NewByName should not have returned an error and has")

			want := testutils.LoadAndUpdateFromGolden(t, got)
			require.Equal(t, want, got, "Shadow object is the expected one")
		})
	}
}

func TestNextEntry(t *testing.T) {
	tests := map[string]struct {
		numNextIteration int
		hasNoUser        bool
		noIterationInit  bool

		wantEndErrType error
	}{
		"get all users":                     {numNextIteration: 3, wantEndErrType: nss.ErrNotFoundENoEnt},
		"no user in db does not fail":       {hasNoUser: true, numNextIteration: 0, wantEndErrType: nss.ErrNotFoundENoEnt},
		"partial iteration then ends works": {numNextIteration: 1, wantEndErrType: nil},

		// error cases
		"error on iteration not being initialized first": {noIterationInit: true, numNextIteration: 0, wantEndErrType: nss.ErrUnavailableENoEnt},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			cacheDir := t.TempDir()
			if !tc.hasNoUser {
				testutils.CopyDBAndFixPermissions(t, "../testdata/users_in_db", cacheDir)
			}

			uid, gid := testutils.GetCurrentUIDGID(t)
			opts := []cache.Option{cache.WithCacheDir(cacheDir), cache.WithRootUID(uid), cache.WithRootGID(gid), cache.WithShadowGID(gid)}
			shadow.SetCacheOption(opts...)

			if !tc.noIterationInit {
				err := shadow.StartEntryIteration(context.Background())
				require.NoError(t, err, "StartEntryIteration should succeed")
				defer shadow.EndEntryIteration(context.Background())
			}

			var got []shadow.Shadow
			for i := 0; i < tc.numNextIteration; i++ {
				p, err := shadow.NextEntry(context.Background())
				require.NoError(t, err, "Should return users without any errors")
				got = append(got, p)
			}
			_, err := shadow.NextEntry(context.Background())
			if tc.wantEndErrType != nil {
				require.ErrorIs(t, err, tc.wantEndErrType, "Should return ENOENT once there is no more users")
			} else {
				require.NoError(t, err, "We iterated over an existing user and shouldn’t get an error")
			}

			if tc.noIterationInit {
				return // no need to deserialize anything
			}

			want := testutils.LoadAndUpdateFromGolden(t, got)
			if len(want) == 0 {
				want = nil
			}
			require.Equal(t, want, got, "Should list requested users only")
		})
	}
}

func TestStartEndEntryIteration(t *testing.T) {
	tests := map[string]struct {
		alreadyIterationInProgress bool
		noStartIteration           bool
		cacheOpenError             bool

		wantStartIterationErr bool
	}{
		"can start and end iteration":                  {},
		"no error when ending a not started iteration": {noStartIteration: true},

		// error cases
		"error in start when iteration already in progress": {alreadyIterationInProgress: true, wantStartIterationErr: true},
		"error in start when error on opening cache":        {cacheOpenError: true, wantStartIterationErr: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			cacheDir := t.TempDir()

			uid, gid := testutils.GetCurrentUIDGID(t)
			opts := []cache.Option{cache.WithCacheDir(cacheDir), cache.WithRootUID(uid), cache.WithRootGID(gid), cache.WithShadowGID(gid)}
			shadow.SetCacheOption(opts...)

			if tc.alreadyIterationInProgress {
				err := shadow.StartEntryIteration(context.Background())
				require.NoError(t, err, "Setup: first startEntryIteration should have failed by hasn’t")
				defer shadow.EndEntryIteration(context.Background())
			}

			if tc.cacheOpenError {
				opts = append(opts, cache.WithRootUID(4242))
			}
			shadow.SetCacheOption(opts...)

			if !tc.noStartIteration {
				err := shadow.StartEntryIteration(context.Background())
				if tc.wantStartIterationErr {
					require.Error(t, err, "StartEntryIteration should have failed by hasn’t")
					require.ErrorIs(t, err, nss.ErrUnavailableENoEnt, "Error should be of type Unavailable")
					return
				}
				require.NoError(t, err, "StartEntryIteration should have failed by hasn’t")
			}

			err := shadow.EndEntryIteration(context.Background())
			require.NoError(t, err, "EndEntryIteration should never fail but had")
		})
	}
}

func TestRestartIterationWithoutEndingPreviousOne(t *testing.T) {
	cacheDir := t.TempDir()
	testutils.CopyDBAndFixPermissions(t, "../testdata/users_in_db", cacheDir)

	uid, gid := testutils.GetCurrentUIDGID(t)
	opts := []cache.Option{cache.WithCacheDir(cacheDir), cache.WithRootUID(uid), cache.WithRootGID(gid), cache.WithShadowGID(gid)}
	shadow.SetCacheOption(opts...)

	// First iteration group
	err := shadow.StartEntryIteration(context.Background())
	require.NoError(t, err, "StartEntryIteration should succeed")
	defer shadow.EndEntryIteration(context.Background()) // in case of an error in the middle of the test. No-op otherwise

	p, err := shadow.NextEntry(context.Background())
	require.NoError(t, err, "Should return first user without any errors")
	require.NotNil(t, p, "Should return first user")

	err = shadow.EndEntryIteration(context.Background())
	require.NoError(t, err, "EndEntryIteration while iterating should work")

	// Second iteration group
	err = shadow.StartEntryIteration(context.Background())
	require.NoError(t, err, "restart a second entry iteration should succeed")
	defer shadow.EndEntryIteration(context.Background())

	var got []shadow.Shadow
	for i := 0; i < 3; i++ {
		p, err := shadow.NextEntry(context.Background())
		require.NoError(t, err, "Should return users without any errors")
		got = append(got, p)
	}
	_, err = shadow.NextEntry(context.Background())
	require.ErrorIs(t, err, nss.ErrNotFoundENoEnt, "Should return ENOENT once there is no more users")

	want := testutils.LoadAndUpdateFromGolden(t, got)
	if len(want) == 0 {
		want = nil
	}
	require.Equal(t, want, got, "Should list all users from the start")
}

func TestMain(m *testing.M) {
	testutils.InstallUpdateFlag()
	flag.Parse()

	m.Run()
}
