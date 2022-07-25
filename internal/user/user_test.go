package user_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/user"
)

func TestNormalizeName(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		name string
		want string
	}{
		"name with mixed case is lowercase": {name: "fOo@dOmAiN.com", want: "foo@domain.com"},
		"lowercase named is unchanged":      {name: "foo@domain.com", want: "foo@domain.com"},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := user.NormalizeName(tc.name)
			require.Equal(t, tc.want, got, "got expected normalized name")
		})
	}
}
