package passwd_test

import (
	"context"
	"flag"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/cache"
	"github.com/ubuntu/aad-auth/internal/nss/passwd"
	"github.com/ubuntu/aad-auth/internal/testutils"
)

func TestNewByName(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		name string
		noDB bool

		wantErr bool
	}{
		"get existing user by name with encrypted password": {name: "myuser@domain.com"},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cacheDir := t.TempDir()
			if !tc.noDB {
				testutils.CopyDBAndFixPermissions(t, "../testdata/users_in_db", cacheDir)
			}

			uid, gid := testutils.GetCurrentUidGid(t)
			passwd.SetCacheOption(cache.WithCacheDir(cacheDir), cache.WithRootUID(uid), cache.WithRootGID(gid), cache.WithShadowGID(uid))

			got, err := passwd.NewByName(context.Background(), tc.name)
			if tc.wantErr {
				require.Error(t, err, "NewByName should have returned an error and hasn’t")
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
