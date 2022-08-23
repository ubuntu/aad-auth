package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	pamCom "github.com/msteinert/pam"
	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/testutils"
)

var libPath string

// TODO: process coverage once https://github.com/golang/go/issues/51430 is implemented in Go.
func TestPamSmAuthenticate(t *testing.T) {
	uid, gid := testutils.GetCurrentUIDGID(t)

	tests := map[string]struct {
		username            string
		password            string
		conf                string
		initialCache        string
		wrongCacheOwnership bool
		offline             bool

		wantErr bool
	}{
		"authenticate successfully (online)": {},
		"specified offline expiration":       {conf: "withoffline-expiration.conf"},

		// aad.conf with custom homedir and shell values
		"correctly set homedir and shell values for a new user":                                          {conf: "aad-with-homedir-and-shell.conf"},
		"correctly set homedir and shell values specified at domain for a new user with matching domain": {conf: "aad-with-homedir-and-shell-domain.conf"},

		// offline cases
		"offline, connect existing user from cache":                                     {conf: "forceoffline.conf", offline: true, initialCache: "db_with_old_users", username: "futureuser@domain.com"},
		"homedir and shell values should not change for user that was already on cache": {conf: "forceoffline-with-homedir-and-shell.conf", offline: true, initialCache: "db_with_old_users", username: "futureuser@domain.com"},

		// special cases
		"authenticate successfully with unmatched case (online)":                  {username: "Success@Domain.COM"},
		"authenticate successfully on config with values only in matching domain": {conf: "with-domain.conf"},

		// error cases
		"error on invalid conf":                                 {conf: "invalid-aad.conf", wantErr: true},
		"error on unexisting conf":                              {conf: "doesnotexist.conf", wantErr: true},
		"error on unexisting users":                             {username: "no such user", wantErr: true},
		"error on invalid password":                             {username: "invalid credentials", wantErr: true},
		"error on config values only in mismatching domain":     {username: "success@otherdomain.com", conf: "with-domain.conf", wantErr: true},
		"error on offline with user online user not in cache":   {conf: "forceoffline.conf", offline: true, initialCache: "db_with_old_users", wantErr: true},
		"error on offline with purged user accoauthenticateunt": {username: "veryolduser@domain.com", offline: true, initialCache: "db_with_old_users", wantErr: true},
		"error on offline with unpurged old user account":       {conf: "forceoffline-expire-right-away.conf", offline: true, initialCache: "db_with_old_users", username: "veryolduser@domain.com", wantErr: true},
		"error on server error":                                 {username: "unreadable server response", wantErr: true},
		"error on cache can't be created/opened":                {wrongCacheOwnership: true, wantErr: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			if tc.username == "" {
				tc.username = "success@domain.com"
			}
			if tc.password == "" {
				tc.password = "my password"
			}
			if tc.conf == "" {
				tc.conf = "simple-aad.conf"
			}
			tc.conf = filepath.Join("testdata", tc.conf)

			testUID := uid
			if tc.wrongCacheOwnership {
				testUID = 4242
			}

			tmp := t.TempDir()

			// auth-aad config
			pamConfDir := filepath.Join(tmp, "pam.d")
			err := os.MkdirAll(pamConfDir, 0700)
			require.NoError(t, err, "Setup: could not create pam.d temporary directory")

			cacheDir := filepath.Join(tmp, "cache")

			if tc.initialCache != "" {
				testutils.CopyDBAndFixPermissions(t, filepath.Join("testdata", tc.initialCache), cacheDir)
			}

			// pam service configuration
			err = os.WriteFile(filepath.Join(pamConfDir, "aadtest"), []byte(fmt.Sprintf(`
			auth	[success=2 default=ignore]	pam_unix.so nullok debug
			auth    [success=1 default=ignore]  %s conf=%s debug reset logswithdebugonstderr rootUID=%d rootGID=%d shadowGID=%d cachedir=%s mockaad
			auth	requisite			pam_deny.so
			auth	required			pam_permit.so`,
				libPath, tc.conf, testUID, gid, gid, cacheDir)), 0600)
			require.NoError(t, err, "Setup: could not create pam stack config file")

			// pam communication
			start := time.Now().Unix()
			tx, err := pamCom.StartFunc("aadtest", "", func(s pamCom.Style, msg string) (string, error) {
				switch s {
				case pamCom.PromptEchoOn:
					return tc.username, nil
				case pamCom.PromptEchoOff:
					return tc.password, nil
				}

				return "", errors.New("unexpected request")
			}, pamCom.WithConfDir(pamConfDir))
			require.NoError(t, err, "Setup: pam should start a transaction with no error")

			// run pam_sm_authenticate
			err = tx.Authenticate(0)
			if tc.wantErr {
				require.Error(t, err, "Authenticate should have returned an error but did not")
				return
			}
			require.NoError(t, err, "Authenticate should succeed")
			end := time.Now().Unix()

			// Verifies the db permissions
			f, err := os.Stat(filepath.Join(cacheDir, "passwd.db"))
			require.NoError(t, err, "Passwd.db stats must be evaluated")
			// Permission for passwd.db should be 644
			require.Equal(t, "-rw-r--r--", f.Mode().String(), "Passwd does not have the expected permissions (644)")
			f, err = os.Stat(filepath.Join(cacheDir, "shadow.db"))
			require.NoError(t, err, "Shadow.db stats must be evaluated")
			// Permission for shadow.db should be 640
			require.Equal(t, "-rw-r-----", f.Mode().String(), "Shadow does not have the expected permissions (640)")

			dbs := []string{"passwd.db", "shadow.db"}
			// Store the dumps after the authentication
			for _, db := range dbs {
				testutils.SaveAndUpdateDump(t, filepath.Join(cacheDir, db))
			}

			// Save and compare the dumps
			for _, db := range dbs {
				// Handles comparison for online test cases
				if !tc.offline {
					requireEqualDumps(t, filepath.Join("testdata", t.Name(), db+".dump"), filepath.Join(cacheDir, db+".dump"), start, end)
					continue
				}

				// Handles comparison for offline test cases
				want, err := os.ReadFile(filepath.Join("testdata", t.Name(), db+".dump"))
				require.NoError(t, err, "want %s dump must be read", db)

				got, err := os.ReadFile(filepath.Join(cacheDir, db+".dump"))
				require.NoError(t, err, "got %s dump must be read", db)

				require.Equal(t, want, got, "Dumps must match")
			}
		})
	}
}

func TestMain(m *testing.M) {
	testutils.InstallUpdateFlag()
	flag.Parse()
	// Build the pam module in a temporary directory and allow linking to it.
	libDir, cleanup, err := createTempDir()
	if err != nil {
		os.Exit(1)
	}

	libPath = filepath.Join(libDir, "pam_aad.so")
	out, err := exec.Command("go", "build", "-buildmode=c-shared", "-tags", "integrationtests", "-o", libPath).CombinedOutput()
	if err != nil {
		cleanup()
		fmt.Fprintf(os.Stderr, "Can not build pam module (%v) : %s", err, out)
		os.Exit(1)
	}

	m.Run()
}

// createTempDir creates a temporary directory with a cleanup teardown not having a testing.T.
func createTempDir() (tmp string, cleanup func(), err error) {
	if tmp, err = os.MkdirTemp("", "aad-auth-integration-tests-pam"); err != nil {
		fmt.Fprintf(os.Stderr, "Can not create temporary directory %q", tmp)
		return "", nil, err
	}
	return tmp, func() {
		if err := os.RemoveAll(tmp); err != nil {
			fmt.Fprintf(os.Stderr, "Can not clean up temporary directory %q", tmp)
		}
	}, nil
}

func requireEqualDumps(t *testing.T, wantPath, gotPath string, start, end int64) {
	t.Helper()

	want, err := testutils.ReadDumpAsTables(t, wantPath)
	require.NoError(t, err, "Could not read dump file %s", wantPath)

	got, err := testutils.ReadDumpAsTables(t, gotPath)
	require.NoError(t, err, "Could not read dump file %s", gotPath)

	for tableName, wantTable := range want {
		gotTable := got[tableName]
		require.NotNil(t, gotTable, "There should be a table")
		require.Equal(t, len(wantTable.Rows), len(gotTable.Rows), "Tables should have the same number of rows")

		for i, wantRow := range wantTable.Rows {
			gotRow := gotTable.Rows[i]

			for colName, wantData := range wantRow {
				gotData := gotRow[colName]
				require.NotNil(t, gotData, "Got must have the wanted row content")

				// Handles comparison of the columns.
				switch colName {
				// last_online_auth is updated everytime a user logs in (online).
				// Comparion must be done with the time of the test, rather than with the golden dump.
				case "last_online_auth":
					n, _ := strconv.ParseInt(gotData, 10, 64)
					// True if the time of last authentication is between the start and the end of the test.
					x := (start <= n) && (n <= end)
					require.True(t, x, "Time %s (%d) must be between start (%d) and end (%d)", gotData, n, start, end)

				// Handles comparison for most columns.
				default:
					require.Equal(t, wantData, gotData, "Contents of col %s from %s must be the same", colName, tableName)
				}
			}
		}
	}
}
