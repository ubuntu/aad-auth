package nss_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/cache"
	"github.com/ubuntu/aad-auth/internal/nss"
)

func TestConvertErr(t *testing.T) {
	t.Parallel()

	errMsg := "My error"
	tests := map[string]struct {
		origErr error

		wantErrorType error
	}{
		"wrapped ErrTryAgainEAgain error retains original error type":    {origErr: fmt.Errorf("%s. Wrapped: %w", errMsg, nss.ErrTryAgainEAgain), wantErrorType: nss.ErrTryAgainEAgain},
		"wrapped ErrTryAgainERange error retains original error type":    {origErr: fmt.Errorf("%s. Wrapped: %w", errMsg, nss.ErrTryAgainERange), wantErrorType: nss.ErrTryAgainERange},
		"wrapped ErrUnavailableENoEnt error retains original error type": {origErr: fmt.Errorf("%s. Wrapped: %w", errMsg, nss.ErrUnavailableENoEnt), wantErrorType: nss.ErrUnavailableENoEnt},
		"wrapped ErrNotFoundENoEnt error retains original error type":    {origErr: fmt.Errorf("%s. Wrapped: %w", errMsg, nss.ErrNotFoundENoEnt), wantErrorType: nss.ErrNotFoundENoEnt},
		"wrapped ErrNotFoundSuccess error retains original error type":   {origErr: fmt.Errorf("%s. Wrapped: %w", errMsg, nss.ErrNotFoundSuccess), wantErrorType: nss.ErrNotFoundSuccess},

		// special cases
		"wrapped ErrNoEnt error is converted to ErrNotFoundENoEnt": {origErr: fmt.Errorf("%s. Wrapped: %w", errMsg, cache.ErrNoEnt), wantErrorType: nss.ErrNotFoundENoEnt},
		"random error is converted to ErrUnavailableENoEnt":        {origErr: errors.New(errMsg), wantErrorType: nss.ErrUnavailableENoEnt},
		"nil error should return nil":                              {origErr: nil, wantErrorType: nil},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := nss.ConvertErr(tc.origErr)

			if tc.wantErrorType == nil {
				require.NoError(t, err, "Nil input should return nil output")
				return
			}

			assert.Contains(t, err.Error(), errMsg, "Should containing original error message")
			require.True(t, errors.Is(err, tc.wantErrorType), "error (%v) should be of type: %v", err, tc.wantErrorType)
		})
	}
}
