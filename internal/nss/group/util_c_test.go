package group_test

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/nss"
	"github.com/ubuntu/aad-auth/internal/nss/group"
	"github.com/ubuntu/aad-auth/internal/testutils"
)

func TestToCgroup(t *testing.T) {
	t.Parallel()
	g := group.NewTestGroup()

	tests := map[string]struct {
		bufsize int

		wantErr bool
	}{
		"can convert to C group": {bufsize: 100000},

		"can't allocate with buffer too small": {bufsize: 5, wantErr: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := testutils.NewCGroup()
			buf := (*group.CChar)(testutils.AllocCBuffer(t, testutils.CSizeT(tc.bufsize)))

			err := g.ToCgroup(group.CGroup(unsafe.Pointer(got)), buf, group.CSizeT(tc.bufsize))
			if tc.wantErr {
				require.Error(t, err, "ToCgroup should have returned an error but hasn't")
				require.ErrorIs(t, err, nss.ErrTryAgainERange, "Error should be of type ErrTryAgainERange")
				return
			}
			require.NoError(t, err, "ToCgroup should have not returned an error but hasn’t")

			grpGot := got.ToPublicCGroup(1)
			want := testutils.SaveAndLoadFromGolden(t, grpGot)

			require.Equal(t, want, grpGot, "Should have C group with expected fields content")
		})
	}
}
