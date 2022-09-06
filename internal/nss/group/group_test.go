package group_test

import (
	"context"
	"flag"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/cache"
	"github.com/ubuntu/aad-auth/internal/nss"
	"github.com/ubuntu/aad-auth/internal/nss/group"
	"github.com/ubuntu/aad-auth/internal/testutils"
)

//nolint:dupl // TestNewByName and TestNewByGID have similar code that triggers dupl, despite being different.
func TestNewByName(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		name         string
		failingCache bool

		wantErrType error
	}{
		"get existing group by name": {name: "myuser@domain.com"},

		// error cases
		"error on non existing group":                 {name: "notexists@domain.com", wantErrType: nss.ErrNotFoundENoEnt},
		"shadow group is ignored, as not part of aad": {name: "shadow", wantErrType: nss.ErrNotFoundENoEnt},
		"error on cache not available":                {name: "myuser@domain.com", failingCache: true, wantErrType: nss.ErrUnavailableENoEnt},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cacheDir := t.TempDir()
			testutils.PrepareDBsForTests(t, cacheDir, "users_in_db")

			uid, gid := testutils.GetCurrentUIDGID(t)
			opts := []cache.Option{cache.WithCacheDir(cacheDir), cache.WithRootUID(uid), cache.WithRootGID(gid), cache.WithShadowGID(gid)}
			if tc.failingCache {
				opts = append(opts, cache.WithRootUID(4242))
			}

			got, err := group.NewByName(context.Background(), tc.name, opts...)
			if tc.wantErrType != nil {
				require.Error(t, err, "NewByName should have returned an error and hasn’t")
				require.ErrorIs(t, err, tc.wantErrType, "NewByName has not returned expected error type")
				return
			}
			require.NoError(t, err, "NewByName should not have returned an error and has")

			want := testutils.LoadYAMLWithUpdateFromGolden(t, got)
			require.Equal(t, want, got, "Group object is the expected one")
		})
	}
}

//nolint:dupl // TestNewByName and TestNewByGID have similar code that triggers dupl, despite being different.
func TestNewByGID(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		gid          uint
		failingCache bool

		wantErrType error
	}{
		"get existing group by gid": {gid: 1929326240},

		// error cases
		"error on non existing group":  {gid: 4242, wantErrType: nss.ErrNotFoundENoEnt},
		"error on cache not available": {gid: 1929326240, failingCache: true, wantErrType: nss.ErrUnavailableENoEnt},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cacheDir := t.TempDir()
			testutils.PrepareDBsForTests(t, cacheDir, "users_in_db")

			uid, gid := testutils.GetCurrentUIDGID(t)
			opts := []cache.Option{cache.WithCacheDir(cacheDir), cache.WithRootUID(uid), cache.WithRootGID(gid), cache.WithShadowGID(gid)}
			if tc.failingCache {
				opts = append(opts, cache.WithRootUID(4242))
			}

			got, err := group.NewByGID(context.Background(), tc.gid, opts...)
			if tc.wantErrType != nil {
				require.Error(t, err, "NewByGID should have returned an error and hasn’t")
				require.ErrorIs(t, err, tc.wantErrType, "NewByGID has not returned expected error type")
				return
			}
			require.NoError(t, err, "NewByGID should not have returned an error and has")

			want := testutils.LoadYAMLWithUpdateFromGolden(t, got)
			require.Equal(t, want, got, "Group object is the expected one")
		})
	}
}

func TestNextEntry(t *testing.T) {
	tests := map[string]struct {
		numNextIteration int
		hasNoGroup       bool
		noIterationInit  bool

		wantEndErrType error
	}{
		"get all groups":                    {numNextIteration: 3, wantEndErrType: nss.ErrNotFoundENoEnt},
		"no group in db does not fail":      {hasNoGroup: true, numNextIteration: 0, wantEndErrType: nss.ErrNotFoundENoEnt},
		"partial iteration then ends works": {numNextIteration: 1, wantEndErrType: nil},

		// error cases
		"error on iteration not being initialized first": {noIterationInit: true, numNextIteration: 0, wantEndErrType: nss.ErrUnavailableENoEnt},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			cacheDir := t.TempDir()
			if !tc.hasNoGroup {
				testutils.PrepareDBsForTests(t, cacheDir, "users_in_db")
			}

			uid, gid := testutils.GetCurrentUIDGID(t)
			opts := []cache.Option{cache.WithCacheDir(cacheDir), cache.WithRootUID(uid), cache.WithRootGID(gid), cache.WithShadowGID(gid)}

			if !tc.noIterationInit {
				err := group.StartEntryIteration(context.Background(), opts...)
				require.NoError(t, err, "StartEntryIteration should succeed")
				defer group.EndEntryIteration(context.Background())
			}

			var got []group.Group
			for i := 0; i < tc.numNextIteration; i++ {
				p, err := group.NextEntry(context.Background())
				require.NoError(t, err, "Should return groups without any errors")
				got = append(got, p)
			}
			_, err := group.NextEntry(context.Background())
			if tc.wantEndErrType != nil {
				require.ErrorIs(t, err, tc.wantEndErrType, "Should return ENOENT once there is no more groups")
			} else {
				require.NoError(t, err, "We iterated over an existing group and shouldn’t get an error")
			}

			if tc.noIterationInit {
				return // no need to deserialize anything
			}

			want := testutils.LoadYAMLWithUpdateFromGolden(t, got)
			if len(want) == 0 {
				want = nil
			}
			require.Equal(t, want, got, "Should list requested groups only")
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

			if tc.alreadyIterationInProgress {
				err := group.StartEntryIteration(context.Background(), opts...)
				require.NoError(t, err, "Setup: first startEntryIteration should have failed by hasn’t")
				defer group.EndEntryIteration(context.Background())
			}

			if tc.cacheOpenError {
				opts = append(opts, cache.WithRootUID(4242))
			}

			if !tc.noStartIteration {
				err := group.StartEntryIteration(context.Background(), opts...)
				if tc.wantStartIterationErr {
					require.Error(t, err, "StartEntryIteration should have failed by hasn’t")
					require.ErrorIs(t, err, nss.ErrUnavailableENoEnt, "Error should be of type Unavailable")
					return
				}
				require.NoError(t, err, "StartEntryIteration should have failed by hasn’t")
			}

			err := group.EndEntryIteration(context.Background())
			require.NoError(t, err, "EndEntryIteration should never fail but had")
		})
	}
}

func TestRestartIterationWithoutEndingPreviousOne(t *testing.T) {
	cacheDir := t.TempDir()
	testutils.PrepareDBsForTests(t, cacheDir, "users_in_db")

	uid, gid := testutils.GetCurrentUIDGID(t)
	opts := []cache.Option{cache.WithCacheDir(cacheDir), cache.WithRootUID(uid), cache.WithRootGID(gid), cache.WithShadowGID(gid)}

	// First iteration group
	err := group.StartEntryIteration(context.Background(), opts...)
	require.NoError(t, err, "StartEntryIteration should succeed")
	defer group.EndEntryIteration(context.Background()) // in case of an error in the middle of the test. No-op otherwise

	p, err := group.NextEntry(context.Background())
	require.NoError(t, err, "Should return first group without any errors")
	require.NotNil(t, p, "Should return first group")

	err = group.EndEntryIteration(context.Background())
	require.NoError(t, err, "EndEntryIteration while iterating should work")

	// Second iteration group
	err = group.StartEntryIteration(context.Background(), opts...)
	require.NoError(t, err, "restart a second entry iteration should succeed")
	defer group.EndEntryIteration(context.Background())

	var got []group.Group
	for i := 0; i < 3; i++ {
		p, err := group.NextEntry(context.Background())
		require.NoError(t, err, "Should return groups without any errors")
		got = append(got, p)
	}
	_, err = group.NextEntry(context.Background())
	require.ErrorIs(t, err, nss.ErrNotFoundENoEnt, "Should return ENOENT once there is no more groups")

	want := testutils.LoadYAMLWithUpdateFromGolden(t, got)
	if len(want) == 0 {
		want = nil
	}
	require.Equal(t, want, got, "Should list all groups from the start")
}

func TestString(t *testing.T) {
	g := group.NewTestGroup(2)

	got := g.String()
	want := testutils.LoadYAMLWithUpdateFromGolden(t, got)
	require.Equal(t, want, got, "Group strings must match")
}

func TestMain(m *testing.M) {
	testutils.InstallUpdateFlag()
	flag.Parse()

	m.Run()
}
