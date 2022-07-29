package config

import (
	"context"
	"log"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadDefaultHomeAndShell(t *testing.T) {
	testFilesPath := filepath.Join("testdata", "loadDefaultHomeAndShell")
	log.Print(testFilesPath)
	t.Parallel()

	tests := map[string]struct {
		path string

		wantHome  string
		wantShell string
	}{
		"file with both home and shell": {
			path:      "adduser-both-values.conf",
			wantHome:  "/home/users/%u",
			wantShell: "/bin/fish",
		},
		"file with only dhome": {
			path:      "adduser-dhome-only.conf",
			wantHome:  "/home/users/%u",
			wantShell: "/bin/bash",
		},
		"file with only dshell": {
			path:      "adduser-dshell-only.conf",
			wantHome:  "/home/%u",
			wantShell: "/bin/fish",
		},
		"file with no values": {
			path:      "adduser-commented.conf",
			wantHome:  "/home/%u",
			wantShell: "/bin/bash",
		},
		"file does not exists returns empty values": {
			path:      "/foo/doesnotexists.conf",
			wantHome:  "",
			wantShell: "",
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			log.Print(tc.path)
			home, shell := loadDefaultHomeAndShell(context.Background(), tc.path)
			require.Equal(t, tc.wantHome, home, "Got expected homedir")
			require.Equal(t, tc.wantShell, shell, "Got expected shell")
		})
	}

}
