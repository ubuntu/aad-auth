package aad_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/aad"
	"github.com/ubuntu/aad-auth/internal/config"
)

func TestAuthenticate(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		appID    string
		username string

		wantErr error
	}{
		"can authenticate with password only":     {},
		"can authenticate even with mfa required": {username: "requireMFA@domain.com"},

		// error cases
		"can't connect to authority": {appID: "connection failed", wantErr: aad.ErrNoNetwork},
		"public client disallowed":   {appID: "public client disallowed", wantErr: aad.ErrDeny},
		"no tenant-wide consent":     {appID: "no tenant-wide consent", wantErr: aad.ErrDeny},
		"unreadable server response": {username: "unreadable server response", wantErr: aad.ErrDeny},
		"invalid server response":    {username: "invalid server response", wantErr: aad.ErrDeny},
		"invalid credentials":        {username: "invalid credentials", wantErr: aad.ErrDeny},
		"no such user":               {username: "no such user", wantErr: aad.ErrDeny},
		"unknown error code":         {username: "unknown error code", wantErr: aad.ErrDeny},
		"unknown error type":         {username: "unknown error type", wantErr: aad.ErrNoNetwork},

		// multiple error cases
		"multiple errors, first known (here mfa) wins":                 {username: "multiple errors, first known is mfa", wantErr: nil},
		"multiple errors, first known (here invalid credentials) wins": {username: "multiple errors, first known is invalid credential", wantErr: aad.ErrDeny},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if tc.appID == "" {
				tc.appID = "valid"
			}
			if tc.username == "" {
				tc.username = "success@domain.com"
			}

			auth := aad.NewWithMockClient()
			cfg := config.AAD{
				TenantID: "tenant id",
				AppID:    tc.appID,
			}
			err := auth.Authenticate(context.Background(), cfg, tc.username, "password")
			if tc.wantErr != nil {
				require.Error(t, err)
				require.True(t, errors.Is(err, tc.wantErr), "Error should be %v", tc.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
