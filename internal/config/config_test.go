package config

import (
	"context"
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

var update bool

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
			path:      testFilesPath + "/adduser-both-values.conf",
			wantHome:  "/home/users/%u",
			wantShell: "/bin/fish",
		},
		"file with only dhome": {
			path:      testFilesPath + "/adduser-dhome-only.conf",
			wantHome:  "/home/users/%u",
			wantShell: "/bin/bash",
		},
		"file with only dshell": {
			path:      testFilesPath + "/adduser-dshell-only.conf",
			wantHome:  "/home/%u",
			wantShell: "/bin/fish",
		},
		"file with no values": {
			path:      testFilesPath + "/adduser-commented.conf",
			wantHome:  "/home/%u",
			wantShell: "/bin/bash",
		},
		"file does not exists returns hardcoded defaults": {
			path:      "/foo/doesnotexists.conf",
			wantHome:  "/home/%u",
			wantShell: "/bin/bash",
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

func TestLoadConfig(t *testing.T) {
	t.Parallel()
	testFilesPath := filepath.Join("testdata", "LoadConfig")

	tests := map[string]struct {
		aadConfigPath string
		addUserPath   string
		wantErr       bool
	}{
		"aad.conf with all values": {
			aadConfigPath: testFilesPath + "/aad-with-section.conf",
		},
		"aad.conf with no section": {
			aadConfigPath: testFilesPath + "/aad-with-no-section.conf",
		},
		"aad.conf with missing 'homedir' value in section": {
			aadConfigPath: testFilesPath + "/aad-with-missing-homedir-value-section.conf",
		},
		"aad.conf with missing 'homedir' value in section and in 'default'": {
			aadConfigPath: testFilesPath + "/aad-with-missing-homedir-value.conf",
		},
		"aad.conf with missing 'shell' value in section": {
			aadConfigPath: testFilesPath + "/aad-with-missing-shell-value-section.conf",
		},
		"aad.conf with missing 'shell' value in section and in 'default'": {
			aadConfigPath: testFilesPath + "/aad-with-missing-shell-value.conf",
		},
		"aad.conf with missing 'homedir' and 'shell' values in section": {
			aadConfigPath: testFilesPath + "/aad-with-missing-homedirShell-values-section.conf",
		},
		"aad.conf with missing 'homedir' and 'shell' values in section and in 'default'": {
			aadConfigPath: testFilesPath + "/aad-with-missing-homedirShell-values.conf",
		},
		"aad.conf with missing required 'offline_credentials_expiration' value in section": {
			aadConfigPath: testFilesPath + "/aad-with-missing-expiration-value-section.conf",
		},
		"aad.conf with missing required 'tenant_id' value in section": {
			aadConfigPath: testFilesPath + "/aad-with-missing-tenantId-value-section.conf",
		},
		"aad.conf with missing required 'app_id' value in section": {
			aadConfigPath: testFilesPath + "/aad-with-missing-appId-value-section.conf",
		},

		// Special Cases
		"aad.conf with missing 'homedir' and 'shell' values in section and in 'default' and wrong adduser.conf": {
			aadConfigPath: testFilesPath + "/aad-with-missing-homedirShell-values.conf",
			addUserPath:   "/foo/bar/fizzbuzz",
		},
		"aad.conf with invalid 'offline_credentials_expiration' value": {
			aadConfigPath: testFilesPath + "/aad-with-invalid-expiration-value.conf",
		},
		"aad.conf with invalid 'offline_credentials_expiration' value in section": {
			aadConfigPath: testFilesPath + "/aad-with-invalid-expiration-value-section.conf",
		},

		// Err
		"aad.conf does not exist": {
			aadConfigPath: "/foo/bar/fizzbuzz.conf",
			wantErr:       true,
		},
		"aad.conf missing 'tenant_id' value": {
			aadConfigPath: testFilesPath + "/aad-with-missing-tenantId-value.conf",
			wantErr:       true,
		},
		"aad.conf missing 'app_id' value": {
			aadConfigPath: testFilesPath + "/aad-with-missing-appId-value.conf",
			wantErr:       true,
		},
	}

	for name, tc := range tests {
		def := strings.ToLower(strings.ReplaceAll(name, " ", "_"))
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			domain := "domain.com"
			config, err := LoadConfig(context.Background(), tc.aadConfigPath, domain, WithCustomConfPath(tc.addUserPath))
			if tc.wantErr {
				require.Error(t, err, "LoadConfig should have failed, but didn't")
				return
			}

			goldenPath := filepath.Join(testFilesPath, "golden", def)
			if update {
				t.Logf("updating golden file %s", goldenPath)
				data, err := yaml.Marshal(config)
				require.NoError(t, err, "Cannot marshal AADConfig to YAML")
				err = os.WriteFile(goldenPath, data, 0600)
				require.NoError(t, err, "Could not write golden file %s", goldenPath)
			}

			var wantConfig AADConfig
			data, err := os.ReadFile(goldenPath)
			require.NoError(t, err, "Could not read golden file %s", goldenPath)
			err = yaml.Unmarshal(data, &wantConfig)
			require.NoError(t, err, "Could not unmarshal golden file %s content", goldenPath)

			require.Equal(t, wantConfig, config, "Got config and expected config are different")
		})
	}

}

func TestMain(m *testing.M) {
	flag.BoolVar(&update, "update", false, "Updates the golden files")
	flag.Parse()
	m.Run()
}
