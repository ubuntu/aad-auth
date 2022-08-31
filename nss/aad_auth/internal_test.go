package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/nss"
	"github.com/ubuntu/aad-auth/internal/testutils"
)

func TestFmtOutput(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		err error
	}{
		"properly format with error ErrTryAgainEAgain":    {err: nss.ErrTryAgainEAgain},
		"properly format with error ErrTryAgainERange":    {err: nss.ErrTryAgainERange},
		"properly format with error ErrUnavailableENoEnt": {err: nss.ErrUnavailableENoEnt},
		"properly format with error ErrNotFoundENoEnt":    {err: nss.ErrNotFoundENoEnt},
		"properly format with error ErrNotFoundSuccess":   {err: nss.ErrNotFoundSuccess},
		"properly format with unknown Err":                {err: fmt.Errorf("SomeError")},
		"properly format with nil error":                  {err: nil},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			entries := []string{"myuser@domain.com:x:1929326240:1929326240::/home/myuser@domain.com:/bin/bash"}

			got := fmtOutput(context.Background(), entries, tc.err)

			want := testutils.LoadAndUpdateFromGolden(t, got)
			require.Equal(t, want, got, "Formatted output must match")
		})
	}
}
