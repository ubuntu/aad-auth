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

	tests := map[string]struct {
		bufsize  int
		nMembers int

		wantErr bool
	}{
		"can convert group to C group":                   {bufsize: 100000},
		"can convert group with five members to C group": {bufsize: 100000, nMembers: 5},
		"can't allocate with buffer too small":           {bufsize: 5, wantErr: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if tc.nMembers == 0 {
				tc.nMembers = 1
			}
			g := group.NewTestGroup(tc.nMembers)

			got := testutils.NewCGroup()
			buf := (*group.CChar)(testutils.AllocCBuffer(t, testutils.CSizeT(tc.bufsize)))
			//#nosec:G103 - We need to use unsafe.Pointer because Go thinks that testutils._Ctype_struct_group is different than group._Ctype_struct_group
			err := g.ToCgroup(group.CGroup(unsafe.Pointer(got)), buf, group.CSizeT(tc.bufsize))
			if tc.wantErr {
				require.Error(t, err, "ToCgroup should have returned an error but hasn't")
				require.ErrorIs(t, err, nss.ErrTryAgainERange, "Error should be of type ErrTryAgainERange")
				return
			}
			require.NoError(t, err, "ToCgroup should have not returned an error but hasn’t")

			grpGot := got.ToPublicCGroup(tc.nMembers)
			want := testutils.LoadYAMLWithUpdateFromGolden(t, grpGot)

			require.Equal(t, want, grpGot, "Should have C group with expected fields content")
		})
	}
}
