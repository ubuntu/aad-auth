package cache

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/testutils"
)

// NewCacheForTests returns a cache that is closed automatically, with permissions set to current user.
func NewCacheForTests(t *testing.T, cacheDir string, options ...Option) (c *Cache) {
	t.Helper()

	uid, gid := testutils.GetCurrentUIDGID(t)
	opts := append([]Option{}, WithCacheDir(cacheDir),
		WithRootUID(uid), WithRootGID(gid), WithShadowGID(gid))

	opts = append(opts, options...)

	c, err := New(context.Background(), opts...)
	require.NoError(t, err, "Setup: should be able to create a cache")
	t.Cleanup(func() { c.Close(context.Background()) })

	return c
}
