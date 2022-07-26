package cache_test

import (
	"context"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/cache"
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
		"can still open shadow file RO":              {shadowCreationFilePerm: &roFilePerm, wantShadowMode: &cache.ShadowROMode},
		"no access to shadow file is still allowded": {shadowCreationFilePerm: &noAccessFilePerm, wantShadowMode: &cache.ShadowNotAvailableMode},

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
				uid, gid = getCurrentUidGid(t)
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
			require.Equal(t, c.ShadowMode(), wantShadowMode, "Shadow attached mode is not the expected one")

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

	uid, gid := getCurrentUidGid(t)

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

	uid, gid := getCurrentUidGid(t)

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

func TestUpdate(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		shadowMode *int
		userNames  []string

		doRefreshWithShadowMode *int

		wantErr        bool
		wantErrRefresh bool
	}{
		"insert a new user":                   {},
		"insert 2 new users":                  {userNames: []string{"firstuser@domain.com", "seconduser@domain.com"}},
		"we don’t create about the user case": {userNames: []string{"MyUser"}},

		"update an existing user should refresh password and last online login": {doRefreshWithShadowMode: &cache.ShadowRWMode},

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
			c := newCacheForTests(t, cacheDir, true)

			if tc.shadowMode != nil {
				c.SetShadowMode(*tc.shadowMode)
			}

			for _, n := range tc.userNames {
				err := c.Update(context.Background(), n, "my password")
				if tc.wantErr {
					require.Error(t, err, "Update should have returned an error but hasn’t")
					return
				}
				require.NoError(t, err, "Update should not have returned an error but has")

				// Check the user exists in DB
				u, err := c.GetUserByName(context.Background(), n)
				require.NoError(t, err, "GetUserByName should get the user we just inserted")

				firstEncryptedPass := u.ShadowPasswd
				firstOnlineLoginTime := u.LastOnlineAuth

				if tc.doRefreshWithShadowMode == nil {
					continue
				}

				// Close and reload a new cache object to ensure we do reload everything from files
				c.Close(context.Background())
				c.WaitForCacheClosed()
				c = newCacheForTests(t, cacheDir, true)
				c.SetShadowMode(*tc.doRefreshWithShadowMode)

				// we need one second as we are storing an unix timestamp for last online auth
				time.Sleep(time.Second)

				err = c.Update(context.Background(), n, "other password")
				if tc.wantErrRefresh {
					require.Error(t, err, "Second update should have returned an error but hasn’t")
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

func newCacheForTests(t *testing.T, cacheDir string, closeWithoutDelay bool) (c *cache.Cache) {
	t.Helper()

	uid, gid := getCurrentUidGid(t)
	opts := append([]cache.Option{}, cache.WithCacheDir(cacheDir),
		cache.WithRootUID(uid), cache.WithRootGID(gid), cache.WithShadowGID(gid))

	if closeWithoutDelay {
		opts = append(opts, cache.WithTeardownDuration(0))
	}

	c, err := cache.New(context.Background(), opts...)
	require.NoError(t, err, "Setup: should be able to create a cache")
	t.Cleanup(func() { c.Close(context.Background()) })

	return c
}

func getCurrentUidGid(t *testing.T) (int, int) {
	t.Helper()

	u, err := user.Current()
	require.NoError(t, err, "Setup: could not get current user")

	uid, err := strconv.Atoi(u.Uid)
	require.NoError(t, err, "Setup: could not convert current uid")
	gid, err := strconv.Atoi(u.Gid)
	require.NoError(t, err, "Setup: could not convert current gid")

	return uid, gid
}
