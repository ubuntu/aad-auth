package pam

import (
	"context"
	"errors"
	"testing"

	pamCom "github.com/msteinert/pam"
	"github.com/stretchr/testify/require"
)

func TestGetUser(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		noPamContext bool

		want    string
		wantErr bool
	}{
		"got username info": {want: "myuser@domain.com"},

		"error if username can't be retrieved": {noPamContext: true, wantErr: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tx, err := pamCom.StartFunc("aadtest", "", func(s pamCom.Style, msg string) (string, error) {
				switch s {
				case pamCom.PromptEchoOn:
					return "myuser@domain.com", nil
				case pamCom.PromptEchoOff:
					return "MyPassword", nil
				}

				return "", errors.New("unexpected request")
			}, pamCom.WithConfDir("testdata"))
			require.NoError(t, err, "Setup: pam should start a transaction with no error")
			cpam := Handle(tx.Handle)

			err = tx.Authenticate(0)
			require.NoError(t, err, "Setup: Authenticate should not fail as we pam_permit without requiring pam_unix")

			ctx := context.Background()
			if !tc.noPamContext {
				ctx = CtxWithPamh(context.Background(), cpam)
			}

			u, err := getUser(ctx)
			if tc.wantErr {
				require.Error(t, err, "getUser should have errored out but hasn't")
				return
			}
			require.NoError(t, err, "getUser should not have errored out but has")
			require.Equal(t, tc.want, u, "Got expected user")
		})
	}
}

/*
 We are not a pam module, and so, we can’t test getting the password as it’s only available
 when we are in pam_sm_authenticate
func TestGetPassword(t *testing.T) {
}
*/
