package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadDefaultHomeAndShell(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		file string

		wantHome  string
		wantShell string
	}{
		"file accessible":   {file: "/etc/adduser.conf", wantHome: "/home/", wantShell: "/bin/bash"},
		"file innacessible": {file: "adduser.conf", wantHome: "/home/%u", wantShell: "/bin/bash"},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			home, shell := loadDefaultHomeAndShell(context.Background(), tc.file)
			require.Equal(t, tc.wantHome, home, "Got expected homedir")
			require.Equal(t, tc.wantShell, shell, "Got expected shell")
		})
	}
}
