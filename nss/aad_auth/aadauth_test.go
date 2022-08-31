package main

import (
	"context"
	"flag"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/cache"
	"github.com/ubuntu/aad-auth/internal/testutils"
)

func TestGetAllEnt(t *testing.T) {
	tests := map[string]struct {
		db string

		wantErr bool
	}{
		"list all groups from group db": {db: "group"},
		"list all users from passwd db": {db: "passwd"},
		"list all users from shadow db": {db: "shadow"},

		// Error cases
		"error when trying to list from an inexistent db": {db: "dontexist", wantErr: true},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			cacheDir := t.TempDir()

			testutils.PrepareDBsForTests(t, cacheDir, "users_in_db")

			uid, gid := testutils.GetCurrentUIDGID(t)
			opts := []cache.Option{cache.WithCacheDir(cacheDir), cache.WithRootUID(uid), cache.WithRootGID(gid), cache.WithShadowGID(gid)}

			got, err := GetEnt(context.Background(), tc.db, "", opts...)
			if tc.wantErr {
				require.Error(t, err, "Expected an error but got none.")
				return
			}
			require.NoError(t, err, "Expected no error but got one.")

			want := testutils.LoadAndUpdateFromGolden(t, got)

			require.Equal(t, want, got, "Output must match")
		})
	}
}

func TestGetEnt(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		db  string
		key string

		wantErr bool
	}{
		"list user from passwd db by name": {db: "passwd", key: "myuser@domain.com"},
		"list user from passwd db by uid":  {db: "passwd", key: "165119649"},

		"list group from group db by name": {db: "group", key: "myuser@domain.com"},
		"list group from group db by gid":  {db: "passwd", key: "165119649"},

		"list user from shadow db by name": {db: "shadow", key: "myuser@domain.com"},

		// Error cases
		"error on trying to list inexistent user in passwd db": {db: "passwd", key: "doesnotexist@domain.com", wantErr: true},
		"error on trying to list inexistent user in group db":  {db: "group", key: "doesnotexist@domain.com", wantErr: true},
		"error on trying to list inexistent user in shadow db": {db: "shadow", key: "doesnotexist@domain.com", wantErr: true},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cacheDir := t.TempDir()
			testutils.PrepareDBsForTests(t, cacheDir, "users_in_db")

			uid, gid := testutils.GetCurrentUIDGID(t)
			opts := []cache.Option{cache.WithCacheDir(cacheDir), cache.WithRootUID(uid), cache.WithRootGID(gid), cache.WithShadowGID(gid)}

			got, err := GetEnt(context.Background(), tc.db, tc.key, opts...)
			if tc.wantErr {
				require.Error(t, err, "Expected an error but got none.")
				return
			}
			require.NoError(t, err, "Expected no error but got one.")

			want := testutils.LoadAndUpdateFromGolden(t, got)

			require.Equal(t, want, got, "Output must match")
		})
	}
}

func TestMain(m *testing.M) {
	testutils.InstallUpdateFlag()
	flag.Parse()
	m.Run()
}
