package cache_test

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/cache"
	"github.com/ubuntu/aad-auth/internal/testutils"
)

func TestNew(t *testing.T) {
	t.Parallel()

	var noAccessFilePerm fs.FileMode = 0000
	var roFilePerm fs.FileMode = 0400

	tests := map[string]struct {
		reOpenCache  bool
		waitForClose bool

		// permission issues
		isNotRootUIDGID           bool
		cantChownShadowOnCreation bool
		changeFilePerm            string
		shadowCreationFilePerm    *fs.FileMode

		wantShadowMode *int
		wantErr        bool
		wantErrReopen  bool
	}{
		"create cache with all permissions": {},
		"reuse opened cache":                {reOpenCache: true},
		"reuse closed cache (files exists)": {waitForClose: true, reOpenCache: true},

		// Shadow files special cases
		"can still open shadow file RO":             {shadowCreationFilePerm: &roFilePerm, wantShadowMode: &cache.ShadowROMode},
		"no access to shadow file is still allowed": {shadowCreationFilePerm: &noAccessFilePerm, wantShadowMode: &cache.ShadowNotAvailableMode},

		// error cases
		"can't create DB not being root UID or GID": {isNotRootUIDGID: true, wantErr: true},
		"can't create a cache with Shadow group":    {cantChownShadowOnCreation: true, wantErr: true},

		// tempered/permission errors
		"can't open existing cache with wrong passwd permission": {changeFilePerm: cache.PasswdDB, waitForClose: true, reOpenCache: true, wantErrReopen: true},
		"can't open existing cache with wrong shadow permission": {changeFilePerm: cache.ShadowDB, waitForClose: true, reOpenCache: true, wantErrReopen: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cacheDir := t.TempDir()
			uid, gid := 4243, 4243
			// mock current user as having permission to UID/GID
			if !tc.isNotRootUIDGID {
				uid, gid = testutils.GetCurrentUIDGID(t)
			}

			shadowGid := 424242
			if !tc.cantChownShadowOnCreation {
				shadowGid = gid
			}

			opts := append([]cache.Option{}, cache.WithCacheDir(cacheDir),
				cache.WithRootUID(uid), cache.WithRootGID(gid), cache.WithShadowGID(shadowGid))

			if tc.shadowCreationFilePerm != nil {
				opts = append(opts, cache.WithShadowPermission(*tc.shadowCreationFilePerm))
			}

			if tc.waitForClose {
				opts = append(opts, cache.WithTeardownDuration(time.Second*0))
			}

			c, err := cache.New(context.Background(), opts...)
			if tc.wantErr {
				require.Error(t, err, "New should have returned an error but hasn’t")
				return
			}
			require.NoError(t, err, "New should have not returned an error but did")
			c.Close(context.Background())

			wantShadowMode := 2
			if tc.wantShadowMode != nil {
				wantShadowMode = *tc.wantShadowMode
			}
			require.Equal(t, wantShadowMode, c.ShadowMode(), "Shadow attached mode is not the expected one")

			if !tc.reOpenCache {
				return
			}

			// Wait for all files to be closed
			if tc.waitForClose {
				c.WaitForCacheClosed()
			}

			if tc.changeFilePerm != "" {
				require.NoError(t, os.Chmod(filepath.Join(cacheDir, tc.changeFilePerm), 0400), "Setup: could not make file Read Only")
			}

			c2, err := cache.New(context.Background(), opts...)
			if tc.wantErrReopen {
				require.Error(t, err, "New should have returned an error but hasn’t")
				return
			}
			require.NoError(t, err, "New should have not returned an error but did")
			defer c2.Close(context.Background())

			// c and c2 should be the same object
			if !tc.waitForClose {
				require.Equal(t, c2, c, "cache should still be the same object")
				return
			}
			// c2 was a complete new cache, opened only from files
			require.NotEqual(t, c2, c, "cache should be reloaded and recreated from files")
		})
	}
}

func TestCloseCacheRetention(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()

	uid, gid := testutils.GetCurrentUIDGID(t)

	opts := append([]cache.Option{}, cache.WithCacheDir(cacheDir),
		cache.WithRootUID(uid), cache.WithRootGID(gid), cache.WithShadowGID(gid),
		cache.WithTeardownDuration(time.Second*1))

	// First grab
	c, err := cache.New(context.Background(), opts...)
	require.NoError(t, err, "New should have not returned an error but did")

	cleanedUp := make(chan struct{})
	go func() {
		c.WaitForCacheClosed()
		close(cleanedUp)
	}()

	c.Close(context.Background())

	// Second grab
	c2, err := cache.New(context.Background(), opts...)
	require.NoError(t, err, "New should have not returned an error but did")

	require.Equal(t, c2, c, "cache should still be the same object")

	// Ensure the cache is not cleaned up after more than a second
	select {
	case <-cleanedUp:
		t.Fatal("cache was collected while still having one element grabbing it")
	case <-time.After(time.Second * 2):
	}

	// Release second grab
	c2.Close(context.Background())

	select {
	case <-time.After(time.Second * 2):
		t.Fatal("cache was not collected while having no more reference grabbing it")
	case <-cleanedUp:
	}
}

func TestCloseCacheDifferentOptions(t *testing.T) {
	t.Parallel()
	cacheDir1, cacheDir2 := t.TempDir(), t.TempDir()

	uid, gid := testutils.GetCurrentUIDGID(t)

	opts := append([]cache.Option{},
		cache.WithRootUID(uid), cache.WithRootGID(gid), cache.WithShadowGID(gid),
		cache.WithTeardownDuration(time.Second*1))

	// First element
	c1, err := cache.New(context.Background(), append(opts, cache.WithCacheDir(cacheDir1))...)
	require.NoError(t, err, "New should have not returned an error but did")
	defer c1.Close(context.Background())

	// Second element
	c2, err := cache.New(context.Background(), append(opts, cache.WithCacheDir(cacheDir2))...)
	require.NoError(t, err, "New should have not returned an error but did")
	defer c2.Close(context.Background())

	require.NotEqual(t, c1, c2, "cache should be separate elements")
}

func TestCleanupDB(t *testing.T) {
	t.Parallel()

	var zeroDuration int
	offlineAuthDisabled := -1

	tests := map[string]struct {
		offlineCredentialsExpirationTime *int

		wantKeepOldUsers bool
	}{
		"clean up old users":                             {},
		"clean up old users with default cleanup policy": {offlineCredentialsExpirationTime: &offlineAuthDisabled},
		"do not clean up anyone":                         {offlineCredentialsExpirationTime: &zeroDuration, wantKeepOldUsers: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cacheDir := t.TempDir()

			uid, gid := testutils.GetCurrentUIDGID(t)
			opts := append([]cache.Option{}, cache.WithCacheDir(cacheDir),
				cache.WithRootUID(uid), cache.WithRootGID(gid), cache.WithShadowGID(gid))

			if tc.offlineCredentialsExpirationTime != nil {
				opts = append(opts, cache.WithOfflineCredentialsExpiration(*tc.offlineCredentialsExpirationTime))
			}

			testutils.PrepareDBsForTests(t, cacheDir, "db_with_expired_users")

			// This triggers a database cleanup if offlineCredentialsExpirationTime is not 0
			c, err := cache.New(context.Background(), opts...)
			require.NoError(t, err, "Should be able to create a cache and clean up")
			t.Cleanup(func() { c.Close(context.Background()) })

			_, errUserPurged := c.GetUserByName(context.Background(), "purgeduser@domain.com")
			_, errUserExpired := c.GetUserByName(context.Background(), "expireduser@domain.com")
			_, errUserValid := c.GetUserByName(context.Background(), "futureuser@domain.com")

			if tc.wantKeepOldUsers {
				assert.NoError(t, errUserPurged, "Very old user should not be cleaned up due to duration being 0")
				assert.NoError(t, errUserExpired, "Not that old user should not be cleaned up due to duration being 0")
			} else {
				assert.Error(t, errUserPurged, "Very old user should be cleaned up")
				assert.NoError(t, errUserExpired, "Expired user should not be cleaned up")
			}

			assert.NoError(t, errUserValid, "Really recent of valid user should not be cleaned up")
		})
	}
}

func TestUpdate(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		shadowMode *int
		userNames  []string

		doRefreshWithShadowMode *int

		wantErr          bool
		wantErrRefresh   bool
		wantUIDCollision bool
	}{
		"insert a new user":                   {},
		"insert 2 new users":                  {userNames: []string{"firstuser@domain.com", "seconduser@domain.com"}},
		"we don’t create about the user case": {userNames: []string{"MyUser"}},

		"update an existing user should refresh password and last online login": {doRefreshWithShadowMode: &cache.ShadowRWMode},
		"collide generated uids": {userNames: []string{"firstuser@domain.com", "userfirst@domain.com"}, wantUIDCollision: true},

		// error cases
		"can't insert with shadow unavailable Only":                   {shadowMode: &cache.ShadowNotAvailableMode, wantErr: true},
		"can't insert with shadow Read Only":                          {shadowMode: &cache.ShadowROMode, wantErr: true},
		"can't update an existing user failed if no access to shadow": {doRefreshWithShadowMode: &cache.ShadowNotAvailableMode, wantErrRefresh: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if tc.userNames == nil {
				tc.userNames = []string{"myuser@domain.com"}
			}

			// First, try to get  user
			cacheDir := t.TempDir()

			opts := []cache.Option{}
			if tc.shadowMode != nil {
				opts = append(opts, cache.WithShadowMode(*tc.shadowMode))
			}
			c := testutils.NewCacheForTests(t, cacheDir, opts...)

			var lastUID int64
			for _, n := range tc.userNames {
				start := time.Now()
				err := c.Update(context.Background(), n, "my password", "/home/%f", "/bin/bash")
				end := time.Now()
				if tc.wantErr {
					require.Error(t, err, "Update should have returned an error but hasn't")
					return
				}
				require.NoError(t, err, "Update should not have returned an error but has")

				// Check the user exists in DB
				u, err := c.GetUserByName(context.Background(), n)
				require.NoError(t, err, "GetUserByName should get the user we just inserted")

				if lastUID != 0 && tc.wantUIDCollision {
					assert.Equal(t, lastUID+1, u.UID, "Colliding user should have existing user UID+1")
				}
				lastUID = u.UID

				if tc.doRefreshWithShadowMode == nil {
					continue
				}

				require.True(t, testutils.TimeBetweenOrEquals(u.LastOnlineAuth, start, end), "LastOnlineAuth (%s) for the user should be between start (%s) and end (%s)", u.LastOnlineAuth.String(), start.String(), end.String())

				firstEncryptedPass := u.ShadowPasswd
				firstOnlineLoginTime := u.LastOnlineAuth

				// Close and reload a new cache object to ensure we do reload everything from files
				c.Close(context.Background())
				c.WaitForCacheClosed()
				c = testutils.NewCacheForTests(t, cacheDir, cache.WithShadowMode(*tc.doRefreshWithShadowMode))

				// we need one second as we are storing an unix timestamp for last online auth
				time.Sleep(time.Second)

				err = c.Update(context.Background(), n, "other password", "/home/%f", "/bin/bash")
				if tc.wantErrRefresh {
					require.Error(t, err, "Second update should have returned an error but hasn't")
					return
				}
				require.NoError(t, err, "Second update should not have returned an error but has")

				// Get updated user information in DB
				u, err = c.GetUserByName(context.Background(), n)
				require.NoError(t, err, "GetUserByName should get the user we just inserted")

				require.NotEqual(t, u.ShadowPasswd, firstEncryptedPass, "Password should have been updated")
				require.True(t, firstOnlineLoginTime.Before(u.LastOnlineAuth), "Should have updated last login time")
			}
		})
	}
}

func TestCanAuthenticate(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		userPasswords                map[string]string
		withoutCredentialsExpiration bool
		shadowMode                   *int
		initialCache                 string
		disabledOfflineAuth          bool

		wantErr bool
	}{
		"can authenticate one user":                                         {userPasswords: map[string]string{"myuser@domain.com": "my password"}},
		"handle separately multiple users and password":                     {userPasswords: map[string]string{"myuser@domain.com": "my password", "otheruser@domain.com": "other password"}},
		"can authenticate even with shadow file RO":                         {userPasswords: map[string]string{"myuser@domain.com": "my password"}, shadowMode: &cache.ShadowROMode},
		"can authenticate even with expired user if expiration is disabled": {userPasswords: map[string]string{"expireduser@domain.com": "my password"}, withoutCredentialsExpiration: true, initialCache: "db_with_expired_users"},
		"can authenticate even with purged user if expiration is disabled":  {userPasswords: map[string]string{"purgeduser@domain.com": "my password"}, withoutCredentialsExpiration: true, initialCache: "db_with_expired_users"},

		// error cases
		"error on wrong password":                         {userPasswords: map[string]string{"myuser@domain.com": "wrong password"}, wantErr: true},
		"error on wrong user":                             {userPasswords: map[string]string{"does not exist user": "my password"}, wantErr: true},
		"error on checking when can’t access shadow file": {userPasswords: map[string]string{"myuser@domain.com": "my password"}, shadowMode: &cache.ShadowNotAvailableMode, wantErr: true},
		"error on trying to authenticate expired user":    {userPasswords: map[string]string{"expireduser@domain.com": "my password"}, initialCache: "db_with_expired_users", wantErr: true},
		"error on offline authentication disabled":        {userPasswords: map[string]string{"myuser@domain.com": "my password"}, disabledOfflineAuth: true, wantErr: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cacheDir := t.TempDir()

			opts := []cache.Option{}
			if tc.shadowMode != nil {
				opts = append(opts, cache.WithShadowMode(*tc.shadowMode))
			}

			initialCache := "users_in_db"
			if tc.initialCache != "" {
				initialCache = tc.initialCache
			}

			if tc.withoutCredentialsExpiration {
				opts = append(opts, cache.WithOfflineCredentialsExpiration(0))
			}

			if tc.disabledOfflineAuth {
				opts = append(opts, cache.WithOfflineCredentialsExpiration(-1))
			}

			testutils.PrepareDBsForTests(t, cacheDir, initialCache, opts...)

			c := testutils.NewCacheForTests(t, cacheDir, opts...)
			for username, password := range tc.userPasswords {
				err := c.CanAuthenticate(context.Background(), username, password)
				if tc.wantErr {
					require.Error(t, err, "CanAuthenticate should return an error but hasn't")
					if tc.initialCache == "db_with_expired_users" {
						require.ErrorIs(t, err, cache.ErrOfflineCredentialsExpired, "CanAuthenticate should return a certain error type for expired unpurged users")
					}

					if tc.disabledOfflineAuth {
						require.ErrorIs(t, err, cache.ErrOfflineAuthDisabled, "CanAuthenticate should return a certain error type for disabled offline authentication")
					}
					return
				}
				assert.NoError(t, err, "CanAuthenticate should not have returned an error but has")
			}
		})
	}
}

func TestUpdateUserAttribute(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		username  string
		attribute string
		value     any

		wantErr bool
	}{
		"gecos": {attribute: "gecos", value: "new gecos"},
		"home":  {attribute: "home", value: "new home"},
		"shell": {attribute: "shell", value: "new shell"},

		// error cases
		"unsupported attribute": {attribute: "uid", value: 1, wantErr: true},
		"unsupported value":     {attribute: "gecos", value: []string{"a"}, wantErr: true},
		"nonexistent user":      {username: "nonexistentuser@domain.com", attribute: "gecos", wantErr: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if tc.username == "" {
				tc.username = "myuser@domain.com"
			}

			cacheDir := t.TempDir()
			cacheDB := "users_in_db"
			testutils.PrepareDBsForTests(t, cacheDir, cacheDB)
			c := testutils.NewCacheForTests(t, cacheDir)

			var wantTime time.Time
			if tc.username != "nonexistentuser@domain.com" {
				user, err := c.GetUserByName(context.Background(), tc.username)
				require.NoError(t, err, "Expected no error but got one.")
				wantTime = user.LastOnlineAuth
			}

			err := c.UpdateUserAttribute(context.Background(), tc.username, tc.attribute, tc.value)
			if tc.wantErr {
				require.Error(t, err, "UpdateUserAttribute should return an error but hasn't")
				return
			}
			assert.NoError(t, err, "UpdateUserAttribute should not have returned an error but has")

			user, err := c.GetUserByName(context.Background(), tc.username)
			require.NoError(t, err, "Setup: GetUserByName should not have returned an error but has")

			require.Equal(t, wantTime, user.LastOnlineAuth, "Expected last_online_auth to not change.")

			got, err := user.IniString()
			require.NoError(t, err, "Setup: failed to get user representation as ini")
			got = testutils.TimestampToWildcard(t, got, user.LastOnlineAuth)

			want := testutils.LoadWithUpdateFromGolden(t, got)
			require.Equal(t, want, got, "expected output to match golden file")
		})
	}
}

func TestQueryPasswdAttribute(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		username  string
		attribute string

		wantErr bool
	}{
		"get login":            {attribute: "login"},
		"get password":         {attribute: "password"},
		"get uid":              {attribute: "uid"},
		"get gid":              {attribute: "gid"},
		"get gecos":            {attribute: "gecos"},
		"get home":             {attribute: "home"},
		"get shell":            {attribute: "shell"},
		"get last_online_auth": {attribute: "last_online_auth"},

		// error cases
		"get nonexistent user": {username: "nouser@domain.com", attribute: "uid", wantErr: true},
		"get bad_attribute":    {attribute: "bad_attribute", wantErr: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			start := time.Now()

			if tc.username == "" {
				tc.username = "myuser@domain.com"
			}

			cacheDir := t.TempDir()
			cacheDB := "users_in_db"
			testutils.PrepareDBsForTests(t, cacheDir, cacheDB)
			c := testutils.NewCacheForTests(t, cacheDir)

			value, err := c.QueryPasswdAttribute(context.Background(), tc.username, tc.attribute)
			if tc.wantErr {
				require.Error(t, err, "QueryPasswdAttribute should return an error but hasn't")
				return
			}
			assert.NoError(t, err, "QueryPasswdAttribute should not have returned an error but has")

			got := fmt.Sprintf("%#v\n", value)
			if tc.attribute == "last_online_auth" {
				i, ok := value.(int64)
				require.True(t, ok, "Value must be an int64")

				gotTime := time.Unix(i, 0)
				start = start.Add(-48 * time.Hour)
				end := testutils.ParseTimeWildcard("RECENT_TIME")
				require.True(t, testutils.TimeBetweenOrEquals(gotTime, start, end), "Got time does not match wanted time")
				return
			}

			want := testutils.LoadWithUpdateFromGolden(t, got)
			require.Equal(t, want, got, "expected output to match golden file")
		})
	}
}

func TestMain(m *testing.M) {
	testutils.InstallUpdateFlag()
	flag.Parse()

	m.Run()
}
