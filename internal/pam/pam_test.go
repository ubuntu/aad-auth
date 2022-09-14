package pam_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/aad"
	"github.com/ubuntu/aad-auth/internal/cache"
	"github.com/ubuntu/aad-auth/internal/pam"
	"github.com/ubuntu/aad-auth/internal/testutils"
)

func TestAuthenticate(t *testing.T) {
	t.Parallel()

	uid, gid := testutils.GetCurrentUIDGID(t)

	tests := map[string]struct {
		username            string
		password            string
		conf                string
		initialCache        string
		wrongCacheOwnership bool

		wantErrType error
	}{
		"authenticate successfully (online)": {},
		"specified offline expiration":       {conf: "withoffline-expiration.conf"},

		// offline cases
		"Offline, connect existing user from cache": {conf: "forceoffline.conf", initialCache: "users_in_db", username: "myuser@domain.com"},

		// special cases
		"authenticate successfully with unmatched case (online)": {username: "Success@Domain.COM"},

		// error cases
		"error on invalid conf":                               {conf: "invalid-aad.conf", wantErrType: pam.ErrPamSystem},
		"error on unexisting conf":                            {conf: "doesnotexist.conf", wantErrType: pam.ErrPamSystem},
		"error on unexisting users":                           {username: "no such user", wantErrType: pam.ErrPamAuth},
		"error on invalid password":                           {username: "invalid credentials", wantErrType: pam.ErrPamAuth},
		"error on offline with user online user not in cache": {conf: "forceoffline.conf", initialCache: "db_with_expired_users", wantErrType: pam.ErrPamAuth},
		"error on offline with purged user account":           {username: "purgeduser@domain.com", initialCache: "db_with_expired_users", wantErrType: pam.ErrPamAuth},
		"error on offline with expired user account":          {conf: "forceoffline.conf", initialCache: "db_with_expired_users", username: "expireduser@domain.com", wantErrType: pam.ErrPamAuth},
		"error on offline with unpurged old user account":     {conf: "forceoffline-expire-right-away.conf", initialCache: "db_with_expired_users", username: "purgeduser@domain.com", wantErrType: pam.ErrPamAuth},
		"error on server error":                               {username: "unreadable server response", wantErrType: pam.ErrPamAuth},
		"error on cache can't be created/opened":              {wrongCacheOwnership: true, wantErrType: pam.ErrPamSystem},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

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

			cacheDir := t.TempDir()
			if tc.initialCache != "" {
				testutils.PrepareDBsForTests(t, cacheDir, tc.initialCache)
			}

			auth := aad.NewWithMockClient()

			cacheOpts := []cache.Option{cache.WithCacheDir(cacheDir),
				cache.WithRootUID(uid), cache.WithRootGID(gid), cache.WithShadowGID(gid)}
			if tc.wrongCacheOwnership {
				cacheOpts = append(cacheOpts, cache.WithRootUID(4242))
			}

			err := pam.Authenticate(context.Background(), tc.username, tc.password, tc.conf,
				pam.WithAuthenticator(auth),
				pam.WithCacheOptions(cacheOpts))
			if tc.wantErrType != nil {
				require.Error(t, err, "Authenticate should have returned an error but did not")
				require.ErrorIs(t, err, tc.wantErrType, "Authenticate has not returned expected error type")
				return
			}

			require.NoError(t, err, "Authenticate should not have returned an error but did")
		})
	}
}
