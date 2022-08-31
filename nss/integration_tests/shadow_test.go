package main

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/testutils"
)

// TODO: process coverage once https://github.com/golang/go/issues/51430 is implemented in Go.
func TestNssGetShadowByName(t *testing.T) {
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
		// password is anonymized to not trigger pam_unix self-check.
		"list existing shadow user": {},

		"no cache no error on existing local shadow user": {name: "root", cacheDB: "-"},

		// error cases
		"error on no access to shadow":                {shadowMode: &noShadow, wantErr: true},
		"shadow user does not exists":                 {name: "doesnotexist@domain.com", wantErr: true},
		"no cache can't get shadow user":              {cacheDB: "-", wantErr: true},
		"invalid permissions on cache can't get user": {rootUID: 4242, wantErr: true},
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
				testutils.PrepareDBsForTests(t, cacheDir, tc.cacheDB)
			}

			if tc.rootUID == 0 {
				tc.rootUID = uid
			}
			shadowMode := -1
			if tc.shadowMode != nil {
				shadowMode = *tc.shadowMode
			}

			got, err := outNSSCommandForLib(t, tc.rootUID, gid, shadowMode, cacheDir, nil, "getent", "shadow", tc.name)
			if tc.wantErr {
				require.Error(t, err, "getent should have errored out but didn't")
				return
			}
			require.NoError(t, err, "getent should succeed")

			want := testutils.LoadAndUpdateFromGolden(t, got)
			require.Equal(t, want, got, "Should get expected aad shadow user")
		})
	}
}

func TestNssGetShadow(t *testing.T) {
	t.Parallel()

	// No need to check for err on originOut as we don’t necessarily have the right to access them.
	originOut, _ := exec.Command("getent", "shadow").CombinedOutput()

	uid, gid := testutils.GetCurrentUIDGID(t)

	noShadow := 0

	tests := map[string]struct {
		cacheDB string

		rootUID    int
		shadowMode *int
	}{
		"list all shadow users": {},

		// special cases
		"no access to shadow list no users":                  {shadowMode: &noShadow},
		"no cache lists no shadow user":                      {cacheDB: "-"},
		"invalid permissions on cache lists no shadow users": {rootUID: 4242},
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
				testutils.PrepareDBsForTests(t, cacheDir, tc.cacheDB)
			}

			if tc.rootUID == 0 {
				tc.rootUID = uid
			}
			shadowMode := -1
			if tc.shadowMode != nil {
				shadowMode = *tc.shadowMode
			}

			got, err := outNSSCommandForLib(t, tc.rootUID, gid, shadowMode, cacheDir, originOut, "getent", "shadow")
			require.NoError(t, err, "getent should succeed")

			want := testutils.LoadAndUpdateFromGolden(t, got)
			require.Equal(t, want, got, "Should get expected aad shadow users listed")
		})
	}
}
