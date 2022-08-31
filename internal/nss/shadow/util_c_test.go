package shadow_test

import (
	"math"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/nss"
	"github.com/ubuntu/aad-auth/internal/nss/shadow"
	"github.com/ubuntu/aad-auth/internal/testutils"
)

func TestToCshadow(t *testing.T) {
	t.Parallel()
	s := shadow.NewTestShadow()

	tests := map[string]struct {
		bufsize int

		wantErr bool
	}{
		"can convert to C shadow": {bufsize: 100000},

		"can't allocate with buffer too small": {bufsize: 5, wantErr: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := testutils.NewCShadow()
			buf := (*shadow.CChar)(testutils.AllocCBuffer(t, testutils.CSizeT(tc.bufsize)))
			//#nosec:G103 - We need to use unsafe.Pointer because Go thinks that testutils._Ctype_struct_shadow is different than shadow._Ctype_struct_shadow
			err := s.ToCshadow(shadow.CShadow(unsafe.Pointer(got)), buf, shadow.CSizeT(tc.bufsize))
			if tc.wantErr {
				require.Error(t, err, "ToCshadow should have returned an error but hasn't")
				require.ErrorIs(t, err, nss.ErrTryAgainERange, "Error should be of type ErrTryAgainERange")
				return
			}
			require.NoError(t, err, "ToCshadow should have not returned an error but hasn’t")

			shadowGot := got.ToPublicCShadow()
			require.EqualValues(t, uint(math.MaxUint), shadowGot.SpFlag, "sp_flag should be equal to math.MaxUint depending on architecture")
			// Golden file stores the 64-bit representation.
			shadowGot.SpFlag = math.MaxUint64

			want := testutils.LoadYAMLWithUpdateFromGolden(t, shadowGot)

			require.Equal(t, want, shadowGot, "Should have C shadow with expected fields content")
		})
	}
}
