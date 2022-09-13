package passwd_test

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/nss"
	"github.com/ubuntu/aad-auth/internal/nss/passwd"
	"github.com/ubuntu/aad-auth/internal/testutils"
)

func TestToCpasswd(t *testing.T) {
	t.Parallel()
	p := passwd.NewTestPasswd()

	tests := map[string]struct {
		bufsize int

		wantErr bool
	}{
		"can convert to C pwd": {bufsize: 100000},

		"can't allocate with buffer too small": {bufsize: 5, wantErr: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := testutils.NewCPasswd()
			buf := (*passwd.CChar)(testutils.AllocCBuffer(t, testutils.CSizeT(tc.bufsize)))
			//#nosec:G103 - We need to use unsafe.Pointer because Go thinks that testutils._Ctype_struct_passwd is different than passwd._Ctype_struct_passwd
			err := p.ToCpasswd(passwd.CPasswd(unsafe.Pointer(got)), buf, passwd.CSizeT(tc.bufsize))
			if tc.wantErr {
				require.Error(t, err, "ToCpasswd should have returned an error but hasn't")
				require.ErrorIs(t, err, nss.ErrTryAgainERange, "Error should be of type ErrTryAgainERange")
				return
			}
			require.NoError(t, err, "ToCpasswd should have not returned an error but hasn’t")

			pwdGot := got.ToPublicCPasswd()
			want := testutils.LoadYAMLWithUpdateFromGolden(t, pwdGot)

			require.Equal(t, want, pwdGot, "Should have C pwd with expected fields content")
		})
	}
}
