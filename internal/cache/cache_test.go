package cache

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseHomeDir(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		path string

		want string
	}{
		"full path":                  {path: "/home/username", want: "/home/username"},
		"path with arg %u":           {path: "/home/%u", want: "/home/user1"},
		"path with arg %U":           {path: "/home/%U", want: "/home/1"},
		"path with arg %d":           {path: "/home/%d", want: "/home/test.com"},
		"path with arg %f":           {path: "/home/%f", want: "/home/user1@test.com"},
		"path with arg %l":           {path: "/home/%l", want: "/home/u"},
		"path with arg %%":           {path: "/home/user%%test.com", want: "/home/user%test.com"},
		"path with multiple args":    {path: "/home/%d/%l/%u%%%U", want: "/home/test.com/u/user1%1"},
		"path with invalid arg":      {path: "/home/%a", want: ""},
		"path with arg after string": {path: "/home/whyDoThis%u", want: "/home/whyDoThisuser1"},
		"path with wtf":              {path: "/home/%%u-%%d", want: "/home/%u-%d"},
		"did someone hurt you?":      {path: "/%u%U%d%f%l%%%u", want: "/user11test.comuser1@test.comu%user1"},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			username, id := "user1@test.com", "1"

			got, err := parseHomeDir(context.Background(), tc.path, username, id)
			if err != nil {
				require.Equal(t, errNoSuchArg, err, "Got expected 'No such argument error'")
			}
			require.Equal(t, tc.want, got, "Got expected parsed path")
		})
	}
}
