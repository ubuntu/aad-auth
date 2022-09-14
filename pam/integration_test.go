package main

import (
	"bytes"
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
		"offline, connect existing user from cache":                                     {conf: "forceoffline.conf", offline: true, initialCache: "users_in_db", username: "myuser@domain.com"},
		"homedir and shell values should not change for user that was already on cache": {conf: "forceoffline-with-homedir-and-shell.conf", offline: true, initialCache: "users_in_db", username: "myuser@domain.com"},
		"offline, connect expired user from cache":                                      {conf: "forceoffline-no-expiration.conf", offline: true, initialCache: "db_with_expired_users", username: "expireduser@domain.com"},
		"offline, connect purged user from cache":                                       {conf: "forceoffline-no-expiration.conf", offline: true, initialCache: "db_with_expired_users", username: "purgeduser@domain.com"},

		// special cases
		"authenticate successfully with unmatched case (online)":                  {username: "Success@Domain.COM"},
		"authenticate successfully on config with values only in matching domain": {conf: "with-domain.conf"},
		"authenticate successfully on config with offline auth disabled (online)": {conf: "offline-auth-disabled.conf"},

		// error cases
		"error on invalid conf":                               {conf: "invalid-aad.conf", wantErr: true},
		"error on unexisting conf":                            {conf: "doesnotexist.conf", wantErr: true},
		"error on unexisting users":                           {username: "no such user", wantErr: true},
		"error on invalid password":                           {username: "invalid credentials", wantErr: true},
		"error on config values only in mismatching domain":   {username: "success@otherdomain.com", conf: "with-domain.conf", wantErr: true},
		"error on offline with user online user not in cache": {conf: "forceoffline.conf", offline: true, initialCache: "db_with_expired_users", wantErr: true},
		"error on offline with expired user":                  {conf: "forceoffline.conf", offline: true, initialCache: "db_with_expired_users", username: "expireduser@domain.com", wantErr: true},
		"error on offline with purged user":                   {conf: "forceoffline-expire-right-away.conf", offline: true, initialCache: "db_with_expired_users", username: "purgeduser@domain.com", wantErr: true},
		"error on offline with offline auth disabled":         {conf: "forceoffline-offline-auth-disabled.conf", offline: true, initialCache: "users_in_db", username: "myuser@domain.com", password: "my password", wantErr: true},
		"error on server error":                               {username: "unreadable server response", wantErr: true},
		"error on cache can't be created/opened":              {wrongCacheOwnership: true, wantErr: true},
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
				testutils.PrepareDBsForTests(t, cacheDir, tc.initialCache)
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
			start := time.Now()
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
			end := time.Now()

			// Verifies the db permissions
			dbPermissions := map[string]string{"passwd.db": "-rw-r--r--", "shadow.db": "-rw-r-----"}
			for n, p := range dbPermissions {
				f, err := os.Stat(filepath.Join(cacheDir, n))
				require.NoError(t, err, "%s stats must be evaluated", n)
				require.Equal(t, p, f.Mode().String(), "%s does not have the expected permissions (%s)", n, p)
			}

			gots := make(map[string]map[string]testutils.Table)
			wants := make(map[string]map[string]testutils.Table)
			// Store the dumps after the authentication
			for db := range dbPermissions {
				ref := filepath.Join(cacheDir, db)
				wants[db] = testutils.LoadAndUpdateFromGoldenDump(t, ref)

				// Load temporary got to memory
				b := &bytes.Buffer{}
				err = testutils.DumpDb(t, ref, b, false)
				require.NoError(t, err, "Setup: can't deserialize temporary dump")

				gots[db], err = testutils.ReadDumpAsTables(t, b)
				require.NoError(t, err, "Could not read temporary dump file for %s", db)
			}

			// Compare the dumps, handling special fields
			for db := range dbPermissions {
				// Handles comparison for online test cases
				requireEqualDumps(t, wants[db], gots[db], tc.offline, start, end)
			}
		})
	}
}

func requireEqualDumps(t *testing.T, want, got map[string]testutils.Table, offline bool, start, end time.Time) {
	t.Helper()

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
				case "password":
					require.NotEmpty(t, gotData, "password should contain something")

				case "last_online_auth":
					// last_online_auth is updated everytime a user logs in (online).
					// Comparison must be done with the time of the test, rather than with the golden dump.
					n, err := strconv.ParseInt(gotData, 10, 64)
					require.NoError(t, err, "last_online_auth should be a valid timestamp")
					if offline {
						require.False(t, testutils.TimeBetweenOrEquals(time.Unix(n, 0), start, end), "Expected time to not have been changed")
						break
					}
					require.True(t, testutils.TimeBetweenOrEquals(time.Unix(n, 0), start, end), "Expected time to be between start and end")

				default:
					// Handles comparison for most columns.
					require.Equal(t, wantData, gotData, "Contents of col %s from %s must be the same", colName, tableName)
				}
			}
		}
	}
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

func TestMain(m *testing.M) {
	testutils.InstallUpdateFlag()
	flag.Parse()
	// Build the pam module in a temporary directory and allow linking to it.
	libDir, cleanup, err := createTempDir()
	if err != nil {
		os.Exit(1)
	}
	defer cleanup()

	libPath = filepath.Join(libDir, "pam_aad.so")
	// #nosec:G204 - we control the command arguments in tests
	out, err := exec.Command("go", "build", "-buildmode=c-shared", "-tags", "integrationtests", "-o", libPath).CombinedOutput()
	if err != nil {
		cleanup()
		fmt.Fprintf(os.Stderr, "Can not build pam module (%v) : %s", err, out)
		os.Exit(1)
	}

	m.Run()
}
