package main

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/testutils"
)

// TODO: process coverage once https://github.com/golang/go/issues/51430 is implemented in Go.
func TestNssGetPasswdByName(t *testing.T) {
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
		"list existing user": {},
		"access to shadow is not needed to list existing user": {shadowMode: &noShadow},

		"no cache no error on existing local user": {name: "root", cacheDB: "-"},

		// error cases
		"user does not exists":                        {name: "doesnotexist@domain.com", wantErr: true},
		"no cache can't get user":                     {cacheDB: "-", wantErr: true},
		"invalid permissions on cache can't get user": {rootUID: 4242, wantErr: true},
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
				testutils.CopyDBAndFixPermissions(t, filepath.Join("testdata", tc.cacheDB), cacheDir)
			}

			if tc.rootUID == 0 {
				tc.rootUID = uid
			}
			shadowMode := -1
			if tc.shadowMode != nil {
				shadowMode = *tc.shadowMode
			}

			got, err := outNSSCommandForLib(t, tc.rootUID, gid, shadowMode, cacheDir, nil, "getent", "passwd", tc.name)
			if tc.wantErr {
				require.Error(t, err, "getent should have errored out but didn't")
				return
			}
			require.NoError(t, err, "getent should succeed")

			want := testutils.LoadAndUpdateFromGolden(t, got)
			require.Equal(t, want, got, "Should get expected aad user")
		})
	}
}
func TestNssGetPasswdByUID(t *testing.T) {
	t.Parallel()

	uid, gid := testutils.GetCurrentUIDGID(t)

	noShadow := 0

	tests := map[string]struct {
		uid string

		cacheDB    string
		rootUID    int
		shadowMode *int

		wantErr bool
	}{
		"list existing user": {},
		"access to shadow is not needed to list existing user": {shadowMode: &noShadow},

		"no cache no error on existing local user": {uid: "0", cacheDB: "-"},

		// error cases
		"user does not exists":                        {uid: "4242", wantErr: true},
		"no cache can't get user":                     {cacheDB: "-", wantErr: true},
		"invalid permissions on cache can't get user": {rootUID: 4242, wantErr: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cacheDir := t.TempDir()
			if tc.uid == "" {
				tc.uid = "1929326240"
			}
			if tc.cacheDB == "" {
				tc.cacheDB = "users_in_db"
			}
			if tc.cacheDB != "-" {
				testutils.CopyDBAndFixPermissions(t, filepath.Join("testdata", tc.cacheDB), cacheDir)
			}

			if tc.rootUID == 0 {
				tc.rootUID = uid
			}
			shadowMode := -1
			if tc.shadowMode != nil {
				shadowMode = *tc.shadowMode
			}

			got, err := outNSSCommandForLib(t, tc.rootUID, gid, shadowMode, cacheDir, nil, "getent", "passwd", tc.uid)
			if tc.wantErr {
				require.Error(t, err, "getent should have errored out but didn't")
				return
			}
			require.NoError(t, err, "getent should succeed")

			want := testutils.LoadAndUpdateFromGolden(t, got)
			require.Equal(t, want, got, "Should get expected aad user")
		})
	}
}
func TestNssGetPasswd(t *testing.T) {
	t.Parallel()

	originOut, err := exec.Command("getent", "passwd").CombinedOutput()
	require.NoError(t, err, "Setup: can't run getent to get original output from system")

	uid, gid := testutils.GetCurrentUIDGID(t)

	noShadow := 0

	tests := map[string]struct {
		cacheDB string

		rootUID    int
		shadowMode *int
	}{
		"list all users": {},
		"access to shadow is not needed to list users": {shadowMode: &noShadow},

		// special cases
		"no cache lists no user":                      {cacheDB: "-"},
		"invalid permissions on cache lists no users": {rootUID: 4242},
		"old users are cleaned up":                    {cacheDB: "db_with_old_users"},
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
				testutils.CopyDBAndFixPermissions(t, filepath.Join("testdata", tc.cacheDB), cacheDir)
			}

			if tc.rootUID == 0 {
				tc.rootUID = uid
			}
			shadowMode := -1
			if tc.shadowMode != nil {
				shadowMode = *tc.shadowMode
			}

			got, err := outNSSCommandForLib(t, tc.rootUID, gid, shadowMode, cacheDir, originOut, "getent", "passwd")
			require.NoError(t, err, "getent should succeed")

			want := testutils.LoadAndUpdateFromGolden(t, got)
			require.Equal(t, want, got, "Should get expected aad users listed")
		})
	}
}
