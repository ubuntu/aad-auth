package main

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/testutils"
)

func TestNssGetent(t *testing.T) {
	t.Parallel()

	noShadow := 0
	//nolint:dupl // We use the same table for the integration and the package tests.
	tests := map[string]struct {
		db         string
		key        string
		cacheDB    string
		rootUID    int
		shadowMode *int

		wantErr bool
	}{
		// List entry by name
		"list entry from passwd by name":                                 {db: "passwd", key: "myuser@domain.com"},
		"list entry from group by name":                                  {db: "group", key: "myuser@domain.com"},
		"list entry from shadow by name":                                 {db: "shadow", key: "myuser@domain.com"},
		"try to list entry from shadow by name without access to shadow": {db: "shadow", key: "myuser@domain.com", shadowMode: &noShadow, wantErr: true},

		// List entry by UID/GID
		"list entry from passwd by uid":        {db: "passwd", key: "165119649"},
		"list entry from group by gid":         {db: "group", key: "165119649"},
		"try to list entry from shadow by uid": {db: "shadow", key: "165119649", wantErr: true},

		// List entries
		"list entries in passwd": {db: "passwd"},
		"list entries in group":  {db: "group"},
		"list entries in shadow": {db: "shadow"},

		// List entries without access to shadow
		"list entries in passwd without access to shadow": {db: "passwd", shadowMode: &noShadow},
		"list entries in group without access to shadow":  {db: "group", shadowMode: &noShadow},
		"try to list shadow without access to shadow":     {db: "shadow", shadowMode: &noShadow, wantErr: true},

		// Try to list non-existent entry
		"try to list non-existent entry in passwd": {db: "passwd", key: "doesnotexist@domain.com", wantErr: true},
		"try to list non-existent entry in group":  {db: "group", key: "doesnotexist@domain.com", wantErr: true},
		"try to list non-existent entry in shadow": {db: "shadow", key: "doesnotexist@domain.com", wantErr: true},

		// Try to list without cache
		"try to list passwd without cache and no permission to create it": {db: "passwd", cacheDB: "nocache", rootUID: 4242., wantErr: true},
		"try to list group without cache and no permission to create it":  {db: "group", cacheDB: "nocache", rootUID: 4242, wantErr: true},
		"try to list shadow without cache and no permission to create it": {db: "shadow", cacheDB: "nocache", rootUID: 4242, wantErr: true},

		// Try to list with empty cache
		"try to list passwd with empty cache": {db: "passwd", cacheDB: "empty", wantErr: true},
		"try to list group with empty cache":  {db: "group", cacheDB: "empty", wantErr: true},
		"try to list shadow with empty cache": {db: "shadow", cacheDB: "empty", wantErr: true},

		// List local entry without cache
		"list local passwd entry without cache": {db: "passwd", cacheDB: "nocache", key: "0"},
		"list local group entry without cache":  {db: "group", cacheDB: "nocache", key: "0"},
		"list local shadow entry without cache": {db: "shadow", cacheDB: "nocache", key: "root", wantErr: true},

		// Cleans up old entries
		"old entries in passwd are cleaned": {db: "passwd", cacheDB: "db_with_old_users"},
		"old entries in group are cleaned":  {db: "group", cacheDB: "db_with_old_users"},
		"old entries in shadow are cleaned": {db: "shadow", cacheDB: "db_with_old_users"},

		// Try to list without permission on cache
		"try to list passwd without permission on cache": {db: "passwd", rootUID: 4242, wantErr: true},
		"try to list group without permission on cache":  {db: "group", rootUID: 4242, wantErr: true},
		"try to list shadow without permission on cache": {db: "shadow", rootUID: 4242, wantErr: true},

		// Error when trying to list from unsupported database
		"error trying to list entry by name from unsupported db": {db: "unsupported", key: "myuser@domain.com", wantErr: true},
		"error trying to list unsupported db":                    {db: "unsupported", wantErr: true},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			uid, gid := testutils.GetCurrentUIDGID(t)
			if tc.rootUID != 0 {
				uid = tc.rootUID
			}

			cacheDir := t.TempDir()
			switch tc.cacheDB {
			case "":
				testutils.PrepareDBsForTests(t, cacheDir, "users_in_db")
			case "db_with_old_users":
				testutils.PrepareDBsForTests(t, cacheDir, tc.cacheDB)
			case "empty":
				testutils.NewCacheForTests(t, cacheDir)
			case "nocache":
				break
			default:
				t.Fatalf("Unexpected value used for cacheDB: %q", tc.cacheDB)
			}

			shadowMode := -1
			if tc.shadowMode != nil {
				shadowMode = *tc.shadowMode
			}

			got, err := outNSSCommandForLib(t, uid, gid, shadowMode, cacheDir, nil, "getent", tc.db, tc.key)
			if tc.wantErr {
				require.Error(t, err, "Expected an error but got none.")
				return
			}
			require.NoError(t, err, "Expected no error but got one.")

			want := testutils.LoadAndUpdateFromGolden(t, got)
			require.Equal(t, want, got, "Output must match")
		})
	}
}