package main

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/testutils"
)

// TODO: process coverage once https://github.com/golang/go/issues/51430 is implemented in Go.
func TestNssGetGroupByName(t *testing.T) {
	t.Parallel()

	uid, gid := testutils.GetCurrentUIDGID(t)

	noShadow := 0

	tests := map[string]struct {
		name string

		cacheDB    string
		rootUID    int
		shadowMode *int

		wantErr bool
	}{
		"list existing group": {},
		"access to shadow is not needed to list existing group": {shadowMode: &noShadow},

		"no cache no error on existing local group": {name: "root", cacheDB: "-"},

		// error cases
		"group does not exists":                        {name: "doesnotexist@domain.com", wantErr: true},
		"no cache can't get group":                     {cacheDB: "-", wantErr: true},
		"invalid permissions on cache can't get group": {rootUID: 4242, wantErr: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cacheDir := t.TempDir()
			if tc.name == "" {
				tc.name = "myuser@domain.com"
			}
			if tc.cacheDB == "" {
				tc.cacheDB = "users_in_db"
			}
			if tc.cacheDB != "-" {
				testutils.PrepareDBsForTests(t, cacheDir, tc.cacheDB)
			}

			if tc.rootUID == 0 {
				tc.rootUID = uid
			}
			shadowMode := -1
			if tc.shadowMode != nil {
				shadowMode = *tc.shadowMode
			}

			got, err := outNSSCommandForLib(t, tc.rootUID, gid, shadowMode, cacheDir, nil, "getent", "group", tc.name)
			if tc.wantErr {
				require.Error(t, err, "getent should have errored out but didn't")
				return
			}
			require.NoError(t, err, "getent should succeed")

			want := testutils.LoadAndUpdateFromGolden(t, got)
			require.Equal(t, want, got, "Should get expected aad group")
		})
	}
}

func TestNssGetGroupByGID(t *testing.T) {
	t.Parallel()

	uid, gid := testutils.GetCurrentUIDGID(t)

	noShadow := 0

	tests := map[string]struct {
		gid string

		cacheDB    string
		rootUID    int
		shadowMode *int

		wantErr bool
	}{
		"list existing group": {},
		"access to shadow is not needed to list existing group": {shadowMode: &noShadow},

		"no cache no error on existing local group": {gid: "0", cacheDB: "-"},

		// error cases
		"group does not exists":                        {gid: "4242", wantErr: true},
		"no cache can't get group":                     {cacheDB: "-", wantErr: true},
		"invalid permissions on cache can't get group": {rootUID: 4242, wantErr: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cacheDir := t.TempDir()
			if tc.gid == "" {
				tc.gid = "1929326240"
			}
			if tc.cacheDB == "" {
				tc.cacheDB = "users_in_db"
			}
			if tc.cacheDB != "-" {
				testutils.PrepareDBsForTests(t, cacheDir, tc.cacheDB)
			}

			if tc.rootUID == 0 {
				tc.rootUID = uid
			}
			shadowMode := -1
			if tc.shadowMode != nil {
				shadowMode = *tc.shadowMode
			}

			got, err := outNSSCommandForLib(t, tc.rootUID, gid, shadowMode, cacheDir, nil, "getent", "group", tc.gid)
			if tc.wantErr {
				require.Error(t, err, "getent should have errored out but didn't")
				return
			}
			require.NoError(t, err, "getent should succeed")

			want := testutils.LoadAndUpdateFromGolden(t, got)
			require.Equal(t, want, got, "Should get expected aad group")
		})
	}
}

func TestNssGetGroup(t *testing.T) {
	t.Parallel()

	originOut, err := exec.Command("getent", "group").CombinedOutput()
	require.NoError(t, err, "Setup: can't run getent to get original output from system")

	uid, gid := testutils.GetCurrentUIDGID(t)

	noShadow := 0

	tests := map[string]struct {
		cacheDB string

		rootUID    int
		shadowGid  int
		shadowMode *int
	}{
		"list all groups": {},
		"access to shadow is not needed to list groups": {shadowMode: &noShadow},

		// special cases
		"no cache lists no groups":                     {cacheDB: "-"},
		"invalid permissions on cache lists no groups": {rootUID: 4242},
		"old groups are cleaned up":                    {cacheDB: "db_with_old_users"},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cacheDir := t.TempDir()
			if tc.cacheDB == "" {
				tc.cacheDB = "users_in_db"
			}
			if tc.cacheDB != "-" {
				testutils.PrepareDBsForTests(t, cacheDir, tc.cacheDB)
			}

			if tc.rootUID == 0 {
				tc.rootUID = uid
			}
			shadowMode := -1
			if tc.shadowMode != nil {
				shadowMode = *tc.shadowMode
			}

			got, err := outNSSCommandForLib(t, tc.rootUID, gid, shadowMode, cacheDir, originOut, "getent", "group")
			require.NoError(t, err, "getent should succeed")

			want := testutils.LoadAndUpdateFromGolden(t, got)
			require.Equal(t, want, got, "Should get expected aad groups listed")
		})
	}
}
