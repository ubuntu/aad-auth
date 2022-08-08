package cache

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseHomeDir(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		path     string
		username string

		want    string
		wantErr bool
	}{
		"handle %f":                         {path: "/home/%f", want: "/home/user1@test.com"},
		"handle %u":                         {path: "/home/%u", want: "/home/user1"},
		"handle %U":                         {path: "/home/%U", want: "/home/42"},
		"handle %d":                         {path: "/home/%d", want: "/home/test.com"},
		"handle %f without domain attached": {username: "userWithoutDomain", path: "/home/%f", want: "/home/userWithoutDomain"},
		"handle %l":                         {path: "/home/%l", want: "/home/u"},
		"handle %%":                         {path: "/home/user%%test.com", want: "/home/user%test.com"},
		"pattern after string":              {path: "/home/whyDoThis%u", want: "/home/whyDoThisuser1"},

		// multiple patterns
		"multiple consecutive patterns":               {path: "/home/%d/%l/%u%U", want: "/home/test.com/u/user142"},
		"multiple patterns separated with characters": {path: "/home/%u-%d", want: "/home/user1-test.com"},

		// special cases
		"full path without modifier is returned as is": {path: "/home/username", want: "/home/username"},

		// error cases
		"error out on path with invalid pattern": {path: "/home/%a", wantErr: true},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			username, id := "user1@test.com", "42"
			if tc.username != "" {
				username = tc.username
			}

			got, err := parseHomeDir(context.Background(), tc.path, username, id)
			if tc.wantErr {
				require.Error(t, err, "parseHomeDir should have returned an error but did not")
				return
			}
			require.NoError(t, err, "parseHomeDir should have not have errored out but did")
			require.Equal(t, tc.want, got, "Should get expected parsed path but did not")
		})
	}
}
