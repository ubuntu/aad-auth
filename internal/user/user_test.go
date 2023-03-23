package user_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/user"
)

func TestNormalizeName(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		name string
		want string
	}{
		"name with mixed case is lowercase": {name: "fOo@dOmAiN.com", want: "foo@domain.com"},
		"lowercase named is unchanged":      {name: "foo@domain.com", want: "foo@domain.com"},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := user.NormalizeName(tc.name)
			require.Equal(t, tc.want, got, "got expected normalized name")
		})
	}
}

func TestIsBusy(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		chroot  string
		wantErr bool
	}{
		"not in use":              {},
		"in use broken root":      {chroot: "/non/existing/path"},
		"in use different chroot": {chroot: "/etc"},

		"in use by proc":      {wantErr: true},
		"in use by proc task": {wantErr: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if tc.chroot == "" {
				tc.chroot = "/"
			}

			procFs := filepath.Join("testdata", t.Name())
			require.DirExists(t, procFs, "Setup: test data directory doesn't exist")

			t.Cleanup(func() {
				err := os.Remove(filepath.Join(procFs, "1", "root"))
				require.NoError(t, err, "Teardown: failed to remove symlink")
				err = os.Remove(filepath.Join(procFs, "2", "root"))
				require.NoError(t, err, "Teardown: failed to remove symlink")
			})

			// First process is always in our namespace
			err := os.Symlink("/", filepath.Join(procFs, "1", "root"))
			require.NoError(t, err, "Setup: failed to create symlink")
			err = os.Symlink(tc.chroot, filepath.Join(procFs, "2", "root"))
			require.NoError(t, err, "Setup: failed to create symlink")

			err = user.IsBusy(procFs, 1000)
			if tc.wantErr {
				require.Error(t, err, "expected error but got none")
				return
			}
			require.NoError(t, err, "got unexpected error")
		})
	}
}
