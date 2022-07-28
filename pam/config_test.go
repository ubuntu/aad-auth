package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadDefaultHomeAndShell(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		path string

		wantHome  string
		wantShell string
	}{
		"file with both home and shell":                   {path: "testdata/adduser-both-values.conf", wantHome: "/home/users/%u", wantShell: "/bin/fish"},
		"file with only dhome":                            {path: "testdata/adduser-dhome-only.conf", wantHome: "/home/users/%u", wantShell: "/bin/bash"},
		"file with only dshell":                           {path: "testdata/adduser-dshell-only.conf", wantHome: "/home/%u", wantShell: "/bin/fish"},
		"file with no values":                             {path: "testdata/adduser-commented.conf", wantHome: "/home/%u", wantShell: "/bin/bash"},
		"file does not exists returns hardcoded defaults": {path: "/foo/doesnotexists.conf", wantHome: "/home/%u", wantShell: "/bin/bash"},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			home, shell := loadDefaultHomeAndShell(context.Background(), tc.path)
			require.Equal(t, tc.wantHome, home, "Got expected homedir")
			require.Equal(t, tc.wantShell, shell, "Got expected shell")
		})
	}
}
