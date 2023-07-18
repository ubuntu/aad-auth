package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/testutils"
)

func TestIntegration(t *testing.T) {
	t.Parallel()

	buildRustNSSLib(t)

	originOuts := make(map[string]string)
	for _, db := range []string{"passwd", "group", "shadow"} {
		//#nosec:G204 - We control the cmd arguments in tests.
		data, err := exec.Command("getent", db).CombinedOutput()
		require.NoError(t, err, "Setup: can't run getent to get original output from system")
		originOuts[db] = string(data)
	}

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
		"list entry from passwd by name":               {db: "passwd", key: "myuser@domain.com"},
		"list entry from passwd with capitalized name": {db: "passwd", key: "MyUser@Domain.Com"},
		"list entry from group by name":                {db: "group", key: "myuser@domain.com"},
		"list entry from group with capitalized name":  {db: "group", key: "MyUser@Domain.Com"},
		"list entry from shadow by name":               {db: "shadow", key: "myuser@domain.com"},
		"list entry from shadow with capitalized name": {db: "shadow", key: "MyUser@Domain.Com"},

		// List entry by UID/GID
		"list entry from passwd by uid":               {db: "passwd", key: "165119649"},
		"list entry from group by gid":                {db: "group", key: "165119649"},
		"error when listing entry from shadow by uid": {db: "shadow", key: "165119649", wantErr: true},

		// List entries
		"list passwd": {db: "passwd"},
		"list group":  {db: "group"},
		"list shadow": {db: "shadow"},

		// List entries without access to shadow
		"list passwd without access to shadow":               {db: "passwd", shadowMode: &noShadow},
		"list group without access to shadow":                {db: "group", shadowMode: &noShadow},
		"returns nothing when listing shadow without access": {db: "shadow", shadowMode: &noShadow},

		// List entries by name without access to shadow
		"list entry from passwd by name without access to shadow":     {db: "passwd", key: "myuser@domain.com", shadowMode: &noShadow},
		"list entry from group by name without access to shadow":      {db: "group", key: "myuser@domain.com", shadowMode: &noShadow},
		"error when listing entry from shadow by name without access": {db: "shadow", key: "myuser@domain.com", shadowMode: &noShadow, wantErr: true},

		// List entries by UID/GID without access to shadow
		"list entry from passwd by uid without access to shadow":     {db: "passwd", key: "165119649", shadowMode: &noShadow},
		"list entry from group by gid without access to shadow":      {db: "group", key: "165119649", shadowMode: &noShadow},
		"error when listing entry from shadow by uid without access": {db: "shadow", key: "165119649", shadowMode: &noShadow, wantErr: true},

		// Error when listing non-existent entry
		"error when listing non-existent entry in passwd": {db: "passwd", key: "doesnotexist@domain.com", wantErr: true},
		"error when listing non-existent entry in group":  {db: "group", key: "doesnotexist@domain.com", wantErr: true},
		"error when listing non-existent entry in shadow": {db: "shadow", key: "doesnotexist@domain.com", wantErr: true},

		// Returns nothing when listing without cache
		"returns nothing when listing passwd without cache and no permission to create it": {db: "passwd", cacheDB: "nocache", rootUID: 4242},
		"returns nothing when listing group without cache and no permission to create it":  {db: "group", cacheDB: "nocache", rootUID: 4242},
		"returns nothing when listing shadow without cache and no permission to create it": {db: "shadow", cacheDB: "nocache", rootUID: 4242},

		// Returns nothing when listing with empty cache
		"returns nothing when listing passwd with empty cache": {db: "passwd", cacheDB: "empty"},
		"returns nothing when listing group with empty cache":  {db: "group", cacheDB: "empty"},
		"returns nothing when listing shadow with empty cache": {db: "shadow", cacheDB: "empty"},

		// List local entry without cache
		"list local passwd entry without cache": {db: "passwd", cacheDB: "nocache", key: "0"},
		"list local group entry without cache":  {db: "group", cacheDB: "nocache", key: "0"},
		"list local shadow entry without cache": {db: "shadow", cacheDB: "nocache", key: "root", wantErr: true},

		// Cleans up old entries
		"old entries in passwd are cleaned": {db: "passwd", cacheDB: "db_with_expired_users"},
		"old entries in group are cleaned":  {db: "group", cacheDB: "db_with_expired_users"},
		"old entries in shadow are cleaned": {db: "shadow", cacheDB: "db_with_expired_users"},

		// Returns nothing when listing without permission on cache
		"returns nothing when listing passwd without permission on cache": {db: "passwd", rootUID: 4242},
		"returns nothing when listing group without permission on cache":  {db: "group", rootUID: 4242},
		"returns nothing when listing shadow without permission on cache": {db: "shadow", rootUID: 4242},

		// Error when trying to list from unsupported database
		"error on trying to list entry by name from unsupported db": {db: "unsupported", key: "myuser@domain.com", wantErr: true},
		"error on trying to list unsupported db":                    {db: "unsupported", wantErr: true},

		// Error when trying to list from db with an explicit empty key
		"error on get entry from passwd with explicit empty key": {db: "passwd", key: "-", wantErr: true},
		"error on get entry from group with explicit empty key":  {db: "group", key: "-", wantErr: true},
		"error on get entry from shadow with explicit empty key": {db: "shadow", key: "-", wantErr: true},
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
			case "db_with_expired_users":
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

			cmds := []string{"getent", tc.db}
			if tc.key == "-" {
				cmds = append(cmds, "")
			} else if tc.key != "" {
				cmds = append(cmds, tc.key)
			}

			got, err := outNSSCommandForLib(t, uid, gid, shadowMode, cacheDir, originOuts[tc.db], cmds...)
			if tc.wantErr {
				require.Error(t, err, "Expected an error but got none: %v", got)
				return
			}
			require.NoError(t, err, "Expected no error but got one: %v", err)

			want := testutils.LoadYAMLWithUpdateFromGolden(t, got)
			require.Equal(t, want, got, "Output must match")
		})
	}
}

func TestMain(m *testing.M) {
	testutils.InstallUpdateFlag()
	flag.Parse()

	code := m.Run()
	if err := testutils.MergeCoverages(); err != nil {
		log.Printf("Teardown: failed to merge coverage files: %v", err)

		// This ensures that we fail the test if we can't merge the coverage files, if the test
		// was successful, otherwise we exit with the code returned by m.Run()
		if code == 0 {
			defer os.Exit(24)
		}
	}
}
