package config

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadDefaultHomeAndShell(t *testing.T) {
	testFilesPath := filepath.Join("testdata", "TestLoadDefaultHomeAndShell")
	t.Parallel()

	tests := map[string]struct {
		path string

		wantHome  string
		wantShell string
	}{
		"file with both home and shell": {
			path:      "adduser-both-values.conf",
			wantHome:  "/home/users/%f",
			wantShell: "/bin/fish",
		},

		"file with only dhome": {
			path:      "adduser-dhome-only.conf",
			wantHome:  "/home/users/%f",
			wantShell: "",
		},
		"file with only dshell": {
			path:      "adduser-dshell-only.conf",
			wantHome:  "",
			wantShell: "/bin/fish",
		},
		"file with no values": {
			path:      "adduser-commented.conf",
			wantHome:  "",
			wantShell: "",
		},

		// Special cases
		"file does not exists returns empty values": {
			path:      "/foo/doesnotexists.conf",
			wantHome:  "",
			wantShell: "",
		},
		"empty path to adduser.conf returns empty values": {
			path:      "",
			wantHome:  "",
			wantShell: "",
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(testFilesPath, tc.path)
			if tc.path == "" {
				path = ""
			}

			home, shell := loadDefaultHomeAndShell(context.Background(), path)
			require.Equal(t, tc.wantHome, home, "Got expected homedir")
			require.Equal(t, tc.wantShell, shell, "Got expected shell")
		})
	}
}
