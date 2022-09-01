package main

import (
	"context"
	"flag"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/cache"
	"github.com/ubuntu/aad-auth/internal/testutils"
)

func TestGetEnt(t *testing.T) {
	noShadow := 0
	tests := map[string]struct {
		db         string
		key        string
		cacheDB    string
		rootUID    int
		shadowMode *int
	}{
		// List entry by name
		"list entry from passwd by name": {db: "passwd", key: "myuser@domain.com"},
		"list entry from group by name":  {db: "group", key: "myuser@domain.com"},
		"list entry from shadow by name": {db: "shadow", key: "myuser@domain.com"},

		// List entry by UID/GID
		"list entry from passwd by uid":        {db: "passwd", key: "165119649"},
		"list entry from group by gid":         {db: "group", key: "165119649"},
		"try to list entry from shadow by uid": {db: "shadow", key: "165119649"},

		// List entries
		"list entries in passwd": {db: "passwd"},
		"list entries in group":  {db: "group"},
		"list entries in shadow": {db: "shadow"},

		// List entries without access to shadow
		"list entries in passwd without access to shadow": {db: "passwd", shadowMode: &noShadow},
		"list entries in group without access to shadow":  {db: "group", shadowMode: &noShadow},
		"try to list shadow without access to shadow":     {db: "shadow", shadowMode: &noShadow},

		// Try to list non-existent entry
		"try to list non-existent entry in passwd": {db: "passwd", key: "doesnotexist@domain.com"},
		"try to list non-existent entry in group":  {db: "group", key: "doesnotexist@domain.com"},
		"try to list non-existent entry in shadow": {db: "shadow", key: "doesnotexist@domain.com"},

		// Try to list without cache
		"try to list passwd without any cache": {db: "passwd", cacheDB: "nocache"},
		"try to list group without any cache":  {db: "group", cacheDB: "nocache"},
		"try to list shadow without any cache": {db: "shadow", cacheDB: "nocache"},

		// Try to list with empty cache
		"try to list passwd with empty cache": {db: "passwd", cacheDB: "empty"},
		"try to list group with empty cache":  {db: "group", cacheDB: "empty"},
		"try to list shadow with empty cache": {db: "shadow", cacheDB: "empty"},

		// List local entry without cache
		"list local passwd entry without cache": {db: "passwd", cacheDB: "nocache", key: "0"},
		"list local group entry without cache":  {db: "group", cacheDB: "nocache", key: "0"},
		"list local shadow entry without cache": {db: "shadow", cacheDB: "nocache", key: "root"},

		// Cleans up old entries
		"old entries in passwd are cleaned": {db: "passwd", cacheDB: "db_with_old_users"},
		"old entries in group are cleaned":  {db: "group", cacheDB: "db_with_old_users"},
		"old entries in shadow are cleaned": {db: "shadow", cacheDB: "db_with_old_users"},

		// Try to list without permission on cache
		"try to list passwd without permission on cache": {db: "passwd", rootUID: 4242},
		"try to list group without permission on cache":  {db: "group", rootUID: 4242},
		"try to list shadow without permission on cache": {db: "shadow", rootUID: 4242},
	}

	// Setting the DB that is not changed which will be used in most tests.
	defaultCacheDir := t.TempDir()
	testutils.PrepareDBsForTests(t, defaultCacheDir, "users_in_db")

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			uid, gid := testutils.GetCurrentUIDGID(t)
			if tc.rootUID != 0 {
				uid = tc.rootUID
			}

			cacheDir := defaultCacheDir
			switch tc.cacheDB {
			case "db_with_old_users":
				cacheDir = t.TempDir()
				testutils.PrepareDBsForTests(t, cacheDir, tc.cacheDB)
			case "empty":
				cacheDir = t.TempDir()
				testutils.NewCacheForTests(t, cacheDir)
			case "nocache":
				cacheDir = t.TempDir()
			}

			opts := []cache.Option{cache.WithCacheDir(cacheDir), cache.WithRootUID(uid), cache.WithRootGID(gid), cache.WithShadowGID(gid)}

			if tc.shadowMode != nil {
				opts = append(opts, cache.WithShadowMode(*tc.shadowMode))
			}

			got := Getent(context.Background(), tc.db, tc.key, opts...)

			want := testutils.LoadAndUpdateFromGolden(t, got)
			require.Equal(t, want, got, "Output must match")
		})
	}
}

func TestMain(m *testing.M) {
	testutils.InstallUpdateFlag()
	flag.Parse()
	m.Run()
}
