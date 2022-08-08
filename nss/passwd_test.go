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

	uid, gid := testutils.GetCurrentUidGid(t)

	tests := map[string]struct {
		name string

		cacheDB   string
		rootUid   int
		shadowGid int

		wantErr bool
	}{
		"list existing user": {},
		"access to shadow is not needed to list existing user": {shadowGid: 4242},

		"no cache no error on existing local user": {name: "root", cacheDB: "-"},

		// error cases
		"user does not exists":                        {name: "doesnotexist@domain.com", wantErr: true},
		"no cache can't get user":                     {cacheDB: "-", wantErr: true},
		"invalid permissions on cache can't get user": {rootUid: 4242, wantErr: true},
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

			if tc.rootUid == 0 {
				tc.rootUid = uid
			}
			if tc.shadowGid == 0 {
				tc.shadowGid = gid
			}

			got, err := outNSSCommandForLib(t, tc.rootUid, gid, tc.shadowGid, cacheDir, nil, "getent", "passwd", tc.name)
			if tc.wantErr {
				require.Error(t, err, "getent should have errored out but didn't")
				return
			}
			require.NoError(t, err, "getent should succeed")

			want := testutils.SaveAndLoadFromGolden(t, got)
			require.Equal(t, want, got, "Should get expected aad user")
		})
	}
}
func TestNssGetPasswdByUID(t *testing.T) {
	t.Parallel()

	uid, gid := testutils.GetCurrentUidGid(t)

	tests := map[string]struct {
		uid string

		cacheDB   string
		rootUid   int
		shadowGid int

		wantErr bool
	}{
		"list existing user": {},
		"access to shadow is not needed to list existing user": {shadowGid: 4242},

		"no cache no error on existing local user": {uid: "0", cacheDB: "-"},

		// error cases
		"user does not exists":                        {uid: "4242", wantErr: true},
		"no cache can't get user":                     {cacheDB: "-", wantErr: true},
		"invalid permissions on cache can't get user": {rootUid: 4242, wantErr: true},
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

			if tc.rootUid == 0 {
				tc.rootUid = uid
			}
			if tc.shadowGid == 0 {
				tc.shadowGid = gid
			}

			got, err := outNSSCommandForLib(t, tc.rootUid, gid, tc.shadowGid, cacheDir, nil, "getent", "passwd", tc.uid)
			if tc.wantErr {
				require.Error(t, err, "getent should have errored out but didn't")
				return
			}
			require.NoError(t, err, "getent should succeed")

			want := testutils.SaveAndLoadFromGolden(t, got)
			require.Equal(t, want, got, "Should get expected aad user")
		})
	}
}
func TestNssGetPasswd(t *testing.T) {
	t.Parallel()

	originOut, err := exec.Command("getent", "passwd").CombinedOutput()
	require.NoError(t, err, "Setup: can't run getent to get original output from system")

	uid, gid := testutils.GetCurrentUidGid(t)

	tests := map[string]struct {
		cacheDB string

		rootUid   int
		shadowGid int
	}{
		"list all users": {},
		"access to shadow is not needed to list users": {shadowGid: 4242},

		// special cases
		"no cache lists no user":                      {cacheDB: "-"},
		"invalid permissions on cache lists no users": {rootUid: 4242},
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

			if tc.rootUid == 0 {
				tc.rootUid = uid
			}
			if tc.shadowGid == 0 {
				tc.shadowGid = gid
			}

			got, err := outNSSCommandForLib(t, tc.rootUid, gid, tc.shadowGid, cacheDir, originOut, "getent", "passwd")
			require.NoError(t, err, "getent should succeed")

			want := testutils.SaveAndLoadFromGolden(t, got)
			require.Equal(t, want, got, "Should get expected aad users listed")
		})
	}
}
