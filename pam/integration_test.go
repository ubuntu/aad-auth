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

		wantErr bool
	}{
		"authenticate successfully (online)": {},
		"specified offline expiration":       {conf: "withoffline-expiration.conf"},

		// aad.conf with custom homedir and shell values
		"correctly set homedir and shell values for a new user":                                          {conf: "aad-with-homedir-and-shell.conf"},
		"correctly set homedir and shell values specified at domain for a new user with matching domain": {conf: "aad-with-homedir-and-shell-domain.conf"},

		// offline cases
		"offline, connect existing user from cache":                                     {conf: "forceoffline.conf", initialCache: "db_with_old_users", username: "futureuser@domain.com"},
		"homedir and shell values should not change for user that was already on cache": {conf: "forceoffline-with-homedir-and-shell.conf", initialCache: "db_with_old_users", username: "futureuser@domain.com"},

		// special cases
		"authenticate successfully with unmatched case (online)": {username: "Success@Domain.COM"},
		// TODO: Remove matching-domain.conf and replace
		// I think this should be one file (with-domain.conf) and the input of the test should select the matching and mismatching domains.
		// -> I think we can change the mock if the need for @domain.com is annoying.
		"authenticate successfully on config with values only in matching domain": {conf: "matching-domain.conf"},

		// error cases
		"error on invalid conf":                                 {conf: "invalid-aad.conf", wantErr: true},
		"error on unexisting conf":                              {conf: "doesnotexist.conf", wantErr: true},
		"error on unexisting users":                             {username: "no such user", wantErr: true},
		"error on invalid password":                             {username: "invalid credentials", wantErr: true},
		"error on config values only in mismatching domain":     {conf: "mismatching-domain.conf", wantErr: true},
		"error on offline with user online user not in cache":   {conf: "forceoffline.conf", initialCache: "db_with_old_users", wantErr: true},
		"error on offline with purged user accoauthenticateunt": {username: "veryolduser@domain.com", initialCache: "db_with_old_users", wantErr: true},
		"error on offline with unpurged old user account":       {conf: "forceoffline-expire-right-away.conf", initialCache: "db_with_old_users", username: "veryolduser@domain.com", wantErr: true},
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

			dbs := []string{"passwd", "shadow"}
			// Store the dumps after the authentication
			for _, db := range dbs {
				testutils.SaveAndUpdateDump(t, filepath.Join(cacheDir, db+".db"))
			}

			// Compare the dumps
			for _, db := range dbs {
				requireEqualDumps(t, filepath.Join("testdata", t.Name(), db+".db.dump"), filepath.Join(cacheDir, db+".db.dump"))
			}

			// TODO: Ensure the dbs have the right permissions.
			// The integration tests should check the permissions of the files passwd/shadow
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

func requireEqualDumps(t *testing.T, wantPath, gotPath string) {
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

				// Handles comparison of special columns
				switch colName {
				// last_online_auth is updated everytime a user logs in, so comparison should be done with the current time
				// TODO: special case the last online auth when saving it to a well known time at write time.

				// TODO: start, end, compare that the field is between start and end.
				case "last_online_auth":
					n, _ := strconv.ParseInt(gotData, 10, 64)
					timeElapsed := time.Now().Unix() - n
					require.LessOrEqual(t, timeElapsed, int64(60), "Difference must be less than or equal to 60 (sec)")

				// Passwords in shadow.db are rehashed when a user logins in. How to compare them?
				// TODO: special case password at writing time, replace it with something like HASHED_PASSWORD
				case "password":
					continue

				// Handles comparison for most columns
				default:
					require.Equal(t, wantData, gotData, "Contents of col %s from %s should be the same", colName, tableName)
				}
			}
		}
	}
}
