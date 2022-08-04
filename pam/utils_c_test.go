package main

import (
	"errors"
	"testing"

	pamCom "github.com/msteinert/pam"
	"github.com/stretchr/testify/require"
)

func TestGetUser(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		want    string
		wantErr bool
	}{
		"got username info": {want: "myuser@domain.com"},

		// we can't simulate no user return without pam authenticate failing.
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tx, err := pamCom.StartFunc("aadtest-simple", "", func(s pamCom.Style, msg string) (string, error) {
				switch s {
				case pamCom.PromptEchoOn:
					return "myuser@domain.com", nil
				case pamCom.PromptEchoOff:
					return "MyPassword", nil
				}

				return "", errors.New("unexpected request")
			}, pamCom.WithConfDir("testdata"))
			require.NoError(t, err, "Setup: pam should start a transaction with no error")
			cpam := pamHandle(tx.Handle)

			err = tx.Authenticate(0)
			require.NoError(t, err, "Setup: Authenticate should not fail as we pam_permit without requiring pam_unix")

			u, err := getUser(cpam)
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
