package logger

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeMsg(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		format string
		a      string
		want   string
	}{
		"msg will always end by EOL": {format: "My %s", a: "message", want: "My message\n"},
		"msg with EOL is unchanged":  {format: "My %s with EOL\n", a: "message", want: "My message with EOL\n"},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := normalizeMsg(tc.format, tc.a)
			require.Equal(t, tc.want, got, "got expected message with EOL")
		})
	}
}
