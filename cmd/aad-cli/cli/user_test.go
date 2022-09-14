package cli_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
		"get attributes for overridden user":   {args: "user --name myuser@domain.com"},
		"default completion for last argument": {args: "user gecos"},
		"default completion, overridden user":  {args: "user gecos --name myuser@domain.com"},
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
		"get user":                       {args: "--name myuser@domain.com"},
		"get user, shadow not available": {args: "--name myuser@domain.com", shadowNotAvailable: true},

		"get login":            {args: "--name myuser@domain.com login"},
		"get password":         {args: "--name myuser@domain.com password"},
		"get uid":              {args: "--name myuser@domain.com uid"},
		"get gid":              {args: "--name myuser@domain.com gid"},
		"get gecos":            {args: "--name myuser@domain.com gecos"},
		"get home":             {args: "--name myuser@domain.com home"},
		"get shell":            {args: "--name myuser@domain.com shell"},
		"get last_online_auth": {args: "--name myuser@domain.com last_online_auth"},
		"get shadow_password":  {args: "--name myuser@domain.com shadow_password"},

		// error cases
		"get nonexistent user":                      {args: "--name nouser@domain.com", wantErr: true},
		"get bad_attribute":                         {args: "--name myuser@domain.com bad_attribute", wantErr: true},
		"get shadow_password, shadow not available": {args: "--name myuser@domain.com shadow_password", shadowNotAvailable: true, wantErr: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			args := []string{"user"}
			args = append(args, strings.Split(tc.args, " ")...)

			cacheDir := t.TempDir()
			cacheDB := "users_in_db"
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

				if len(args) < 4 || slices.Contains(args, "last_online_auth") {
					tmp := strings.Index(got, user.LastOnlineAuth.Format(time.RFC3339))
					require.NotEqual(t, -1, tmp, "Expected to find the correct time")
				}

				got = testutils.TimestampToWildcard(t, got, user.LastOnlineAuth)
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
		"set gecos":                 {args: "user --name myuser@domain.com gecos newvalue"},
		"set home":                  {args: "user --name myuser@domain.com home newvalue"},
		"set shell":                 {args: "user --name myuser@domain.com shell newvalue"},
		"set shell on default user": {args: "user shell newvalue"},

		// error cases
		"set bad_attribute":    {args: "user --name myuser@domain.com bad_attribute newvalue", wantErr: true},
		"set nonexistent user": {args: "user --name nouser@domain.com gecos newvalue", wantErr: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			args := strings.Split(tc.args, " ")

			usernameIndex := slices.Index(args, "--name")
			username := args[usernameIndex+1]

			// Fallback when a username is not provided
			if usernameIndex == -1 {
				username = "myuser@domain.com"
			}

			cacheDir := t.TempDir()
			cacheDB := "users_in_db"
			testutils.PrepareDBsForTests(t, cacheDir, cacheDB)
			cache := testutils.NewCacheForTests(t, cacheDir)
			c := cli.New(cli.WithCache(cache), cli.WithCurrentUser("myuser@domain.com"))

			// Gets the user time before running the cli
			var wantTime time.Time
			if username != "nouser@domain.com" {
				aux, err := cache.GetUserByName(context.Background(), username)
				require.NoError(t, err, "Expected no error but got one.")
				wantTime = aux.LastOnlineAuth
			}

			_, err := testutils.RunApp(t, c, args...)
			if tc.wantErr {
				require.Error(t, err, "expected command to return an error")
				return
			}
			require.NoError(t, err, "expected command to succeed")

			user, err := cache.GetUserByName(context.Background(), username)
			require.NoError(t, err, "Setup: failed to get user from cache")

			// Handles time comparison separately
			require.Equal(t, wantTime, user.LastOnlineAuth, "Expected last_online_auth to not have changed")

			got, err := user.IniString()
			require.NoError(t, err, "Setup: failed to get user representation as ini")
			got = testutils.TimestampToWildcard(t, got, user.LastOnlineAuth)

			want := testutils.LoadWithUpdateFromGolden(t, got)
			require.Equal(t, want, got, "expected output to match golden file")
		})
	}
}

func TestUserMoveHomeDirectory(t *testing.T) {
	tests := map[string]struct {
		prevHomeDir  string
		newHomeDir   string
		userLoggedIn bool

		wantErr bool
	}{
		"move home directory": {prevHomeDir: "oldhome", newHomeDir: "newhome"},

		// Error cases - homedir attribute is updated
		"fail if previous directory is absent": {prevHomeDir: "absent", newHomeDir: "newhome", wantErr: true},
		"fail if previous directory is a file": {prevHomeDir: "oldhomefile", newHomeDir: "newhome", wantErr: true},
		"fail if new directory already exists": {prevHomeDir: "oldhome", newHomeDir: "existingnewhome", wantErr: true},

		// Error cases - homedir attribute is not updated
		"fail if the user has open processses": {prevHomeDir: "oldhome", newHomeDir: "newhome", userLoggedIn: true, wantErr: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			tmpDir := t.TempDir()
			cacheDir := filepath.Join(tmpDir, "cache")
			testutils.PrepareDBsForTests(t, cacheDir, "db_with_expired_users")
			cache := testutils.NewCacheForTests(t, cacheDir)

			// Set up test filesystem structure
			err := os.MkdirAll(filepath.Join(tmpDir, "oldhome"), 0750)
			require.NoError(t, err, "Setup: failed to create previous home directory")
			err = os.MkdirAll(filepath.Join(tmpDir, "existingnewhome"), 0750)
			require.NoError(t, err, "Setup: failed to create existing new home directory")
			err = os.WriteFile(filepath.Join(tmpDir, "oldhomefile"), []byte("test content"), 0600)
			require.NoError(t, err, "Setup: failed to create previous home directory file")

			// Set up fake /proc structure for checking if the user has open processes
			procFs := filepath.Join("testdata", "not_in_use")
			if tc.userLoggedIn {
				procFs = filepath.Join("testdata", "in_use")
			}

			require.DirExists(t, procFs, "Setup: failed to find fake /proc filesystem")
			t.Cleanup(func() {
				err := os.Remove(filepath.Join(procFs, "1", "root"))
				require.NoError(t, err, "Teardown: failed to remove symlink")
				err = os.Remove(filepath.Join(procFs, "2", "root"))
				require.NoError(t, err, "Teardown: failed to remove symlink")
			})

			// Both processes run in our namespace
			err = os.Symlink("/", filepath.Join(procFs, "1", "root"))
			require.NoError(t, err, "Setup: failed to create symlink")
			err = os.Symlink("/", filepath.Join(procFs, "2", "root"))
			require.NoError(t, err, "Setup: failed to create symlink")

			prevHomeDir := filepath.Join(tmpDir, tc.prevHomeDir)
			newHomeDir := filepath.Join(tmpDir, tc.newHomeDir)

			err = cache.UpdateUserAttribute(context.Background(), "futureuser@domain.com", "home", prevHomeDir)
			require.NoError(t, err, "Setup: failed to set initial user home directory")

			c := cli.New(cli.WithCache(cache), cli.WithProcFs(procFs))
			_, runErr := testutils.RunApp(t, c, "user", "--name", "futureuser@domain.com", "home", newHomeDir, "--move-home")

			// We always expect the passwd attribute to be updated in this test, unless the user has open processes
			home, err := cache.QueryPasswdAttribute(context.Background(), "futureuser@domain.com", "home")
			require.NoError(t, err, "Setup: failed to get user home directory")
			if tc.userLoggedIn {
				require.Equal(t, prevHomeDir, home, "expected home directory not to be updated")
			} else {
				require.Equal(t, newHomeDir, home, "expected home directory to be updated")
			}

			if !tc.wantErr {
				require.NoError(t, runErr, "expected command to succeed")
				require.DirExists(t, newHomeDir, "expected new home directory to exist")
				require.NoDirExists(t, prevHomeDir, "expected previous home directory to not exist")
				return
			}

			require.Error(t, runErr, "expected command to return an error")
			if tc.prevHomeDir == "oldhome" {
				require.DirExists(t, prevHomeDir, "expected previous home directory to exist")
			}
			if tc.newHomeDir != "existingnewhome" {
				require.NoDirExists(t, newHomeDir, "expected new home directory to not exist")
			}
		})
	}
}

func TestUserMutuallyExclusiveFlags(t *testing.T) {
	tests := map[string]struct {
		args        string
		expectedErr string
	}{
		"both --name and --all": {
			args:        "user --name myuser@domain.com --all",
			expectedErr: "if any flags in the group [name all] are set none of the others can be",
		},
		"both -n and -a": {
			args:        "user -n myuser@domain.com -a",
			expectedErr: "if any flags in the group [name all] are set none of the others can be",
		},
		"both --move-home and --all": {
			args:        "user --move-home --all home newvalue",
			expectedErr: "if any flags in the group [move-home all] are set none of the others can be",
		},
		"both -m and -a": {
			args:        "user -m -a home newvalue",
			expectedErr: "if any flags in the group [move-home all] are set none of the others can be",
		},
		"--move-home without argument to update": {
			args:        "user --move-home",
			expectedErr: "move-home can only be used when modifying home attribute",
		},
		"--move-home with incorrect argument to update": {
			args:        "user --move-home gecos newvalue",
			expectedErr: "move-home can only be used when modifying home attribute",
		},
		"--move-home without new value to update with": {
			args:        "user --move-home home",
			expectedErr: "move-home can only be used when modifying home attribute",
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			cacheDir := t.TempDir()
			cacheDB := "users_in_db"
			testutils.PrepareDBsForTests(t, cacheDir, cacheDB)
			cache := testutils.NewCacheForTests(t, cacheDir)

			c := cli.New(cli.WithCache(cache))
			_, err := testutils.RunApp(t, c, strings.Split(tc.args, " ")...)

			require.ErrorContains(t, err, tc.expectedErr, "expected command to return flag parsing error")
		})
	}
}
