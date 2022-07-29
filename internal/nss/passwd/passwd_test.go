package passwd_test

import (
	"context"
	"flag"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/cache"
	"github.com/ubuntu/aad-auth/internal/nss"
	"github.com/ubuntu/aad-auth/internal/nss/passwd"
	"github.com/ubuntu/aad-auth/internal/testutils"
)

func TestNewByName(t *testing.T) {
	tests := map[string]struct {
		name         string
		failingCache bool

		wantErrType error
	}{
		"get existing user by name": {name: "myuser@domain.com"},

		// error cases
		"error on non existing user":   {name: "notexists@domain.com", wantErrType: nss.ErrNotFoundENoEnt},
		"error on cache not available": {name: "myuser@domain.com", failingCache: true, wantErrType: nss.ErrUnavailableENoEnt},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			cacheDir := t.TempDir()
			testutils.CopyDBAndFixPermissions(t, "../testdata/users_in_db", cacheDir)

			uid, gid := testutils.GetCurrentUidGid(t)
			opts := []cache.Option{cache.WithCacheDir(cacheDir), cache.WithRootUID(uid), cache.WithRootGID(gid), cache.WithShadowGID(uid)}
			if tc.failingCache {
				opts = append(opts, cache.WithRootUID(4242))
			}
			passwd.SetCacheOption(opts...)

			got, err := passwd.NewByName(context.Background(), tc.name)
			if tc.wantErrType != nil {
				require.Error(t, err, "NewByName should have returned an error and hasn’t")
				require.ErrorIs(t, err, tc.wantErrType, "NewByName has not returned expected error type")
				return
			}
			require.NoError(t, err, "NewByName should not have returned an error and has")

			want := testutils.SaveAndLoadFromGolden(t, got)
			require.Equal(t, want, got, "Passwd object is the expected one")
		})
	}
}

func TestMain(m *testing.M) {
	testutils.InstallUpdateFlag()
	flag.Parse()

	m.Run()
}
