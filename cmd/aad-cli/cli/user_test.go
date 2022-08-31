package cli_test

import (
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/cmd/aad-cli/cli"
	"github.com/ubuntu/aad-auth/internal/cache"
	"github.com/ubuntu/aad-auth/internal/testutils"
	"golang.org/x/exp/slices"
)

func TestUserShellCompletion(t *testing.T) {
	tests := map[string]struct {
		args string
	}{
		"get all users, short flag":            {args: "user -n"},
		"get all users, long flag":             {args: "user --name"},
		"get attributes for user":              {args: "user"},
		"get attributes for overridden user":   {args: "user --name futureuser@domain.com"},
		"default completion for last argument": {args: "user gecos"},
		"default completion, overridden user":  {args: "user gecos --name futureuser@domain.com"},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			args := []string{cobra.ShellCompRequestCmd}
			args = append(args, strings.Split(tc.args, " ")...)
			args = append(args, "")

			cacheDir := t.TempDir()
			cacheDB := "users_in_db"
			testutils.PrepareDBsForTests(t, cacheDir, cacheDB)
			cache := testutils.NewCacheForTests(t, cacheDir)

			c := cli.New(cli.WithCache(cache))
			got, err := testutils.RunApp(t, c, args...)
			require.NoError(t, err, "failed to run completion")

			want := testutils.LoadWithUpdateFromGolden(t, got)
			require.Equal(t, want, got, "expected output to match golden file")
		})
	}
}

func TestUser(t *testing.T) {
	tests := map[string]struct {
		args               string
		shadowNotAvailable bool

		wantErr bool
	}{
		"get all users":                  {args: "--all"},
		"get user":                       {args: "--name futureuser@domain.com"},
		"get user, shadow not available": {args: "--name futureuser@domain.com", shadowNotAvailable: true},

		"get login":            {args: "--name futureuser@domain.com login"},
		"get password":         {args: "--name futureuser@domain.com password"},
		"get uid":              {args: "--name futureuser@domain.com uid"},
		"get gid":              {args: "--name futureuser@domain.com gid"},
		"get gecos":            {args: "--name futureuser@domain.com gecos"},
		"get home":             {args: "--name futureuser@domain.com home"},
		"get shell":            {args: "--name futureuser@domain.com shell"},
		"get last_online_auth": {args: "--name futureuser@domain.com last_online_auth"},
		"get shadow_password":  {args: "--name futureuser@domain.com shadow_password"},

		// error cases
		"get nonexistent user":                      {args: "--name nouser@domain.com", wantErr: true},
		"get bad_attribute":                         {args: "--name futureuser@domain.com bad_attribute", wantErr: true},
		"get shadow_password, shadow not available": {args: "--name futureuser@domain.com shadow_password", shadowNotAvailable: true, wantErr: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			args := []string{"user"}
			args = append(args, strings.Split(tc.args, " ")...)

			cacheDir := t.TempDir()
			cacheDB := "db_with_old_users"
			testutils.PrepareDBsForTests(t, cacheDir, cacheDB)

			shadowMode := -1
			if tc.shadowNotAvailable {
				shadowMode = 0
			}
			cache := testutils.NewCacheForTests(t, cacheDir, cache.WithShadowMode(shadowMode))
			c := cli.New(cli.WithCache(cache))

			got, err := testutils.RunApp(t, c, args...)
			if tc.wantErr {
				require.Error(t, err, "expected command to return an error")
				return
			}
			require.NoError(t, err, "expected command to succeed")

			if slices.Contains(args, "--name") {
				username := args[slices.Index(args, "--name")+1]
				user, err := cache.GetUserByName(context.Background(), username)
				require.NoError(t, err, "Setup: failed to get user from cache")
				// Timestamps get serialized as RFC3339 which includes timezone
				// information.
				// We replace this with the unix timestamp to make
				// the comparison easier.
				got = testutils.TimestampToUnix(t, got, user.LastOnlineAuth)
			}

			want := testutils.LoadWithUpdateFromGolden(t, got)
			require.Equal(t, want, got, "expected output to match golden file")
		})
	}
}

func TestUserSetAttribute(t *testing.T) {
	tests := map[string]struct {
		args string

		badPerms bool
		wantErr  bool
	}{
		"set gecos": {args: "user --name futureuser@domain.com gecos newvalue"},
		"set home":  {args: "user --name futureuser@domain.com home newvalue"},
		"set shell": {args: "user --name futureuser@domain.com shell newvalue"},

		// error cases
		"set bad_attribute":    {args: "user --name futureuser@domain.com bad_attribute newvalue", wantErr: true},
		"set nonexistent user": {args: "user --name nouser@domain.com gecos newvalue", wantErr: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			args := strings.Split(tc.args, " ")
			username := args[slices.Index(args, "--name")+1]

			cacheDir := t.TempDir()
			cacheDB := "db_with_old_users"
			testutils.PrepareDBsForTests(t, cacheDir, cacheDB)
			cache := testutils.NewCacheForTests(t, cacheDir)
			c := cli.New(cli.WithCache(cache))

			_, err := testutils.RunApp(t, c, args...)
			if tc.wantErr {
				require.Error(t, err, "expected command to return an error")
				return
			}
			require.NoError(t, err, "expected command to succeed")

			user, err := cache.GetUserByName(context.Background(), username)
			require.NoError(t, err, "Setup: failed to get user from cache")
			got, err := user.IniString()
			require.NoError(t, err, "Setup: failed to get user representation as ini")
			got = testutils.TimestampToUnix(t, got, user.LastOnlineAuth)

			want := testutils.LoadWithUpdateFromGolden(t, got)
			require.Equal(t, want, got, "expected output to match golden file")
		})
	}
}

func TestUserMutuallyExclusiveFlags(t *testing.T) {
	cacheDir := t.TempDir()
	cacheDB := "db_with_old_users"
	testutils.PrepareDBsForTests(t, cacheDir, cacheDB)
	cache := testutils.NewCacheForTests(t, cacheDir)
	c := cli.New(cli.WithCache(cache))

	_, err := testutils.RunApp(t, c, "user", "--name", "futureuser@domain.com", "--all")
	require.ErrorContains(t, err, "if any flags in the group [name all] are set none of the others can be", "expected command to return mutually exclusive flag error")

	_, err = testutils.RunApp(t, c, "user", "-n", "futureuser@domain.com", "-a")
	require.ErrorContains(t, err, "if any flags in the group [name all] are set none of the others can be", "expected command to return mutually exclusive flag error")
}
