package testutils

import (
	"os/user"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// GetCurrentUidGid return current uid/gid for the user running the tests.
func GetCurrentUidGid(t *testing.T) (int, int) {
	t.Helper()

	u, err := user.Current()
	require.NoError(t, err, "Setup: could not get current user")

	uid, err := strconv.Atoi(u.Uid)
	require.NoError(t, err, "Setup: could not convert current uid")
	gid, err := strconv.Atoi(u.Gid)
	require.NoError(t, err, "Setup: could not convert current gid")

	return uid, gid
}
