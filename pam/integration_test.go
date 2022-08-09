package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

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

		// offline cases
		"Offline, connect existing user from cache": {conf: "forceoffline.conf", initialCache: "db_with_old_users", username: "futureuser@domain.com"},

		// special cases
		"authenticate successfully with unmatched case (online)": {username: "Success@Domain.COM"},
		// TODO: Should have use cases for per domain configuration

		// error cases
		"error on invalid conf":                               {conf: "invalid-aad.conf", wantErr: true},
		"error on unexisting conf":                            {conf: "doesnotexist.conf", wantErr: true},
		"error on unexisting users":                           {username: "no such user", wantErr: true},
		"error on invalid password":                           {username: "invalid credentials", wantErr: true},
		"error on offline with user online user not in cache": {conf: "forceoffline.conf", initialCache: "db_with_old_users", wantErr: true},
		"error on offline with purged user account":           {username: "veryolduser@domain.com", initialCache: "db_with_old_users", wantErr: true},
		"error on offline with unpurged old user account":     {conf: "forceoffline-expire-right-away.conf", initialCache: "db_with_old_users", username: "veryolduser@domain.com", wantErr: true},
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
		})
	}
}

func TestMain(m *testing.M) {
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

// createTempDir to create a temporary directory with a cleanup teardown not having a testing.T
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
