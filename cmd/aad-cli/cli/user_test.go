package cli_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/cmd/aad-cli/cli"
	"github.com/ubuntu/aad-auth/internal/cache"
	"github.com/ubuntu/aad-auth/internal/testutils"
)

func TestUserShellCompletion(t *testing.T) {
	tests := map[string]struct {
		args []string
	}{
		// user get
		"get all users for get":   {args: []string{"user", "get"}},
		"get attributes for user": {args: []string{"user", "get", "myuser@domain.com"}},
		"no more get completion":  {args: []string{"user", "get", "myuser@domain.com", "gecos"}},

		// user set
		"get all users for set":                {args: []string{"user", "set"}},
		"get attributes for set":               {args: []string{"user", "set", "myuser@domain.com"}},
		"default completion for last argument": {args: []string{"user", "set", "myuser@domain.com", "gecos"}},
		"no more set completion":               {args: []string{"user", "set", "myuser@domain.com", "gecos", "mycomment"}},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			args := []string{cobra.ShellCompRequestCmd}
			args = append(args, tc.args...)
			args = append(args, "")

			cacheDir := t.TempDir()
			testutils.CopyDBAndFixPermissions(t, filepath.Join("testdata", "cachedb"), cacheDir)

			uid, gid := testutils.GetCurrentUIDGID(t)

			cache, err := cache.New(context.Background(), cache.WithCacheDir(cacheDir), cache.WithRootUID(uid), cache.WithRootGID(gid), cache.WithShadowGID(gid))
			require.NoError(t, err, "failed to create cache")

			c := cli.New(cli.WithCache(cache))
			got, err := runApp(t, c, args...)
			require.NoError(t, err, "failed to run completion")

			want := testutils.SaveAndLoadFromGolden(t, got)
			require.Equal(t, want, got, "expected output to match golden file")
		})
	}
}

func TestUserGet(t *testing.T) {
	tests := map[string]struct {
		username           string
		attribute          string
		shadowNotAvailable bool

		wantErr bool
	}{
		"get all users":                  {},
		"get user":                       {username: "myuser@domain.com"},
		"get user, shadow not available": {username: "myuser@domain.com", shadowNotAvailable: true},

		"get login":            {username: "myuser@domain.com", attribute: "login"},
		"get password":         {username: "myuser@domain.com", attribute: "password"},
		"get uid":              {username: "myuser@domain.com", attribute: "uid"},
		"get gid":              {username: "myuser@domain.com", attribute: "gid"},
		"get gecos":            {username: "myuser@domain.com", attribute: "gecos"},
		"get home":             {username: "myuser@domain.com", attribute: "home"},
		"get shell":            {username: "myuser@domain.com", attribute: "shell"},
		"get last_online_auth": {username: "myuser@domain.com", attribute: "last_online_auth"},
		"get shadow_password":  {username: "myuser@domain.com", attribute: "shadow_password"},

		// error cases
		"get nonexistent user":                      {username: "nouser@domain.com", wantErr: true},
		"get bad_attribute":                         {username: "myuser@domain.com", attribute: "bad_attribute", wantErr: true},
		"get shadow_password, shadow not available": {username: "myuser@domain.com", attribute: "shadow_password", shadowNotAvailable: true, wantErr: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			args := []string{"user", "get"}
			if tc.username != "" {
				args = append(args, tc.username)
			}
			if tc.attribute != "" {
				args = append(args, tc.attribute)
			}

			cacheDir := t.TempDir()
			testutils.CopyDBAndFixPermissions(t, filepath.Join("testdata", "cachedb"), cacheDir)
			uid, gid := testutils.GetCurrentUIDGID(t)

			shadowMode := -1
			if tc.shadowNotAvailable {
				shadowMode = 0
			}
			cache, err := cache.New(context.Background(), cache.WithCacheDir(cacheDir), cache.WithRootUID(uid), cache.WithRootGID(gid), cache.WithShadowGID(gid), cache.WithShadowMode(shadowMode))
			require.NoError(t, err, "failed to create cache")
			c := cli.New(cli.WithCache(cache))

			got, err := runApp(t, c, args...)
			if tc.wantErr {
				require.Error(t, err, "expected command to return an error")
				return
			}
			require.NoError(t, err, "expected command to succeed")

			want := testutils.SaveAndLoadFromGolden(t, got)
			require.Equal(t, want, got, "expected output to match golden file")
		})
	}
}

func TestUserSet(t *testing.T) {
	tests := map[string]struct {
		username  string
		attribute string

		badPerms bool
		wantErr  bool
	}{
		"set gecos": {attribute: "gecos"},
		"set home":  {attribute: "home"},
		"set shell": {attribute: "shell"},

		// error cases
		"set bad_attribute":    {attribute: "bad_attribute", wantErr: true},
		"set nonexistent user": {username: "nouser@domain.com", attribute: "gecos", wantErr: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			args := []string{"user", "set"}
			if tc.username == "" {
				tc.username = "myuser@domain.com"
			}
			args = append(args, tc.username, tc.attribute, "newvalue")

			cacheDir := t.TempDir()
			testutils.CopyDBAndFixPermissions(t, filepath.Join("testdata", "cachedb"), cacheDir)
			uid, gid := testutils.GetCurrentUIDGID(t)

			cache, err := cache.New(context.Background(), cache.WithCacheDir(cacheDir), cache.WithRootUID(uid), cache.WithRootGID(gid), cache.WithShadowGID(gid))
			require.NoError(t, err, "failed to create cache")

			c := cli.New(cli.WithCache(cache))

			_, err = runApp(t, c, args...)
			if tc.wantErr {
				require.Error(t, err, "expected command to return an error")
				return
			}
			require.NoError(t, err, "expected command to succeed")

			got, err := cache.GetUserByName(context.Background(), tc.username)
			require.NoError(t, err, "failed to get user from cache")

			want := testutils.SaveAndLoadYAMLFromGolden(t, got)
			require.Equal(t, want, got, "should have updated user in cache")
		})
	}
}
