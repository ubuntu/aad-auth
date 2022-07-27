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

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		configPath string

		wantTenantID           string
		wantAppID              string
		wantOfflineCredentials int
		wantHomeDir            string
		wantShell              string
		wantErr                bool
	}{
		"aad.conf with all values": {
			configPath:             "testdata/aad-with-section.conf",
			wantTenantID:           "2",
			wantAppID:              "2",
			wantOfflineCredentials: 180,
			wantHomeDir:            "/home/%d/%l/%u",
			wantShell:              "/bin/someShell",
		},
		"aad.conf with no section": {
			configPath:             "testdata/aad-with-no-section.conf",
			wantTenantID:           "1",
			wantAppID:              "1",
			wantOfflineCredentials: 90,
			wantHomeDir:            "/home/%d/%u",
			wantShell:              "/bin/fish",
		},
		"aad.conf with missing 'homedir' value in section": {
			configPath:             "testdata/aad-with-missing-homedir-value-section.conf",
			wantTenantID:           "2",
			wantAppID:              "2",
			wantOfflineCredentials: 180,
			wantHomeDir:            "/home/%d/%u",
			wantShell:              "/bin/someShell",
		},
		"aad.conf with missing 'homedir' value in section and in 'default'": {
			configPath:             "testdata/aad-with-missing-homedir-value.conf",
			wantTenantID:           "2",
			wantAppID:              "2",
			wantOfflineCredentials: 180,
			wantHomeDir:            "/home/%u",
			wantShell:              "/bin/someShell",
		},
		"aad.conf with missing 'shell' value in section": {
			configPath:             "testdata/aad-with-missing-shell-value-section.conf",
			wantTenantID:           "2",
			wantAppID:              "2",
			wantOfflineCredentials: 180,
			wantHomeDir:            "/home/%d/%l/%u",
			wantShell:              "/bin/fish",
		},
		"aad.conf with missing 'shell' value in section and in 'default'": {
			configPath:             "testdata/aad-with-missing-shell-value.conf",
			wantTenantID:           "2",
			wantAppID:              "2",
			wantOfflineCredentials: 180,
			wantHomeDir:            "/home/%d/%l/%u",
			wantShell:              "/bin/bash",
		},
		"aad.conf with missing 'homedir' and 'shell' values in section": {
			configPath:             "testdata/aad-with-missing-homedirShell-values-section.conf",
			wantTenantID:           "2",
			wantAppID:              "2",
			wantOfflineCredentials: 180,
			wantHomeDir:            "/home/%d/%u",
			wantShell:              "/bin/fish",
		},
		"aad.conf with missing 'homedir' and 'shell' values in section and in 'default'": {
			configPath:             "testdata/aad-with-missing-homedirShell-values.conf",
			wantTenantID:           "2",
			wantAppID:              "2",
			wantOfflineCredentials: 180,
			wantHomeDir:            "/home/%u",
			wantShell:              "/bin/bash",
		},
		"aad.conf with missing required 'offline_credentials_expiration' value in section": {
			configPath:             "testdata/aad-with-missing-expiration-value-section.conf",
			wantTenantID:           "2",
			wantAppID:              "2",
			wantOfflineCredentials: -1,
			wantHomeDir:            "/home/%d/%l/%u",
			wantShell:              "/bin/someShell",
		},

		// Err
		"aad.conf with missing required 'tenant_id' value in section": {
			configPath: "testdata/aad-with-missing-tenantId-value-section.conf",
			wantErr:    true,
		},
		"aad.conf with missing required 'app_id' value in section": {
			configPath: "testdata/aad-with-missing-appId-value-section.conf",
			wantErr:    true,
		},
		"aad.conf does not exist": {
			configPath: "/foo/bar/fizzbuzz.conf",
			wantErr:    true,
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			domain := "domain.com"
			tenandID, appID, offlineCredentials, homeDir, shell, err := loadConfig(context.Background(), tc.configPath, domain)

			if tc.wantErr {
				require.Error(t, err, "loadConfig should have returned an error but didn't")
				return
			}
			require.NoError(t, err, "loadConfig shouldn't have errored out but did")

			require.Equal(t, tc.wantTenantID, tenandID, "Expected tenantID to be %s, but got %s", tc.wantTenantID, tenandID)
			require.Equal(t, tc.wantAppID, appID, "Expected appID to be %s, but got %s", tc.wantAppID, appID)
			require.Equal(t, tc.wantOfflineCredentials, offlineCredentials, "Expected credentials to be %d, but got %d", tc.wantOfflineCredentials, offlineCredentials)
			require.Equal(t, tc.wantHomeDir, homeDir, "Expected homeDir to be %s, but got %s", tc.wantHomeDir, homeDir)
			require.Equal(t, tc.wantShell, shell, "Expected shell to be %s, but got %s", tc.wantShell, shell)
		})
	}
}
