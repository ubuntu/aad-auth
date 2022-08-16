package testutils

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/termie/go-shutil"
)

// GetCurrentUIDGID return current uid/gid for the user running the tests.
func GetCurrentUIDGID(t *testing.T) (int, int) {
	t.Helper()

	u, err := user.Current()
	require.NoError(t, err, "Setup: could not get current user")

	uid, err := strconv.Atoi(u.Uid)
	require.NoError(t, err, "Setup: could not convert current uid")
	gid, err := strconv.Atoi(u.Gid)
	require.NoError(t, err, "Setup: could not convert current gid")

	return uid, gid
}

// CopyDBAndFixPermissions copies databases in refDir to cacheDir.
// It sets the default expected permissions on those files too.
func CopyDBAndFixPermissions(t *testing.T, refDir, cacheDir string) {
	t.Helper()

	require.NoError(t, os.RemoveAll(cacheDir), "Setup: could not remove to prepare cache directory")
	err := shutil.CopyTree(refDir, cacheDir, nil)
	require.NoError(t, err, "Setup: could not copy initial database files in cache")
	// apply expected permission as git will change them
	// #nosec: G302 - this permission level is required for pam to work.
	require.NoError(t, os.Chmod(filepath.Join(cacheDir, "passwd.db"), 0644), "Setup: failed to set expected permission on passwd db file")
	// #nosec: G302 - this permission level is required for pam to work.
	require.NoError(t, os.Chmod(filepath.Join(cacheDir, "shadow.db"), 0640), "Setup: failed to set expected permission on shadow db file")
}
