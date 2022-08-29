// Package testutils is the package which has helpers for our integration and package tests.
package testutils

import (
	"os/user"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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

// TimeBetweenOrEquals returns if tt is between start and end.
func TimeBetweenOrEquals(tt, start, end time.Time) bool {
	// tt is floor to current second, compare then to the second before start.
	if tt.Before(start.Add(-time.Second)) {
		return false
	}
	if tt.After(end) {
		return false
	}

	return true
}
