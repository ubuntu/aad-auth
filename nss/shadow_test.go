package main

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/testutils"
)

// TODO: process coverage once https://github.com/golang/go/issues/51430 is implemented in Go.
func TestNssGetShadowByName(t *testing.T) {
	t.Parallel()

	uid, gid := testutils.GetCurrentUidGid(t)

	tests := map[string]struct {
		name string

		cacheDB   string
		rootUid   int
		shadowGid int

		wantErr bool
	}{
		// password is anonymized to not trigger pam_unix self-check.
		"list existing shadow user": {},

		"no cache no error on existing local shadow user": {name: "root", cacheDB: "-"},

		// error cases
		"error on no access to shadow":                {shadowGid: 4242, wantErr: true},
		"shadow user does not exists":                 {name: "doesnotexist@domain.com", wantErr: true},
		"no cache can't get shadow user":              {cacheDB: "-", wantErr: true},
		"invalid permissions on cache can't get user": {rootUid: 4242, wantErr: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if tc.name == "root" && uid != 0 {
				t.Skip("can't test getting local user from shadow when not being root or part of shadow group")
			}

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

			got, err := outNSSCommandForLib(t, tc.rootUid, gid, tc.shadowGid, cacheDir, nil, "getent", "shadow", tc.name)
			if tc.wantErr {
				require.Error(t, err, "getent should have errored out but didn't")
				return
			}
			require.NoError(t, err, "getent should succeed")

			want := testutils.SaveAndLoadFromGolden(t, got)
			require.Equal(t, want, got, "Should get expected aad shadow user")
		})
	}
}

func TestNssGetShadow(t *testing.T) {
	t.Parallel()

	// No need to check for err on originOut as we don’t necessarily have the right to access them.
	originOut, _ := exec.Command("getent", "shadow").CombinedOutput()

	uid, gid := testutils.GetCurrentUidGid(t)

	tests := map[string]struct {
		cacheDB string

		rootUid   int
		shadowGid int
	}{
		"list all shadow users": {},

		// special cases
		"no access to shadow list no users":                  {shadowGid: 4242},
		"no cache lists no shadow user":                      {cacheDB: "-"},
		"invalid permissions on cache lists no shadow users": {rootUid: 4242},
		"old shadow users are cleaned up":                    {cacheDB: "db_with_old_users"},
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

			got, err := outNSSCommandForLib(t, tc.rootUid, gid, tc.shadowGid, cacheDir, originOut, "getent", "shadow")
			require.NoError(t, err, "getent should succeed")

			want := testutils.SaveAndLoadFromGolden(t, got)
			require.Equal(t, want, got, "Should get expected aad shadow users listed")
		})
	}
}
