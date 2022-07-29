package config_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/config"
	"github.com/ubuntu/aad-auth/internal/testutils"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()
	testFilesPath := filepath.Join("testdata", "LoadConfig")

	tests := map[string]struct {
		aadConfigPath string
		addUserPath   string
		domain        string

		wantErr bool
	}{

		// All values
		"aad.conf, all values, no domain": {
			aadConfigPath: "aad-all_values-no_domain.conf",
		},
		"aad.conf, all values, with domain": {
			aadConfigPath: "aad-all_values-with_domain.conf",
		},
		"aad.conf, all values, mismatch domain": {
			aadConfigPath: "aad-all_values-with_domain.conf",
			domain:        "doesNotExist.com",
		},
		"aad.conf, all values, only in domain": {
			aadConfigPath: "aad-all_values_only_in_domain.conf",
		},

		// Missing values in domain
		"aad.conf with missing 'homedirpattern' value in domain": {
			aadConfigPath: "aad-missing_homedirpattern-domain.conf",
		},
		"aad.conf with missing 'shell' value in domain": {
			aadConfigPath: "aad-missing_shell-domain.conf",
		},
		"aad.conf with missing 'homedirpattern' and 'shell' values in domain": {
			aadConfigPath: "aad-missing_homedirpattern_and_shell-domain.conf",
		},
		"aad.conf with missing 'offline_credentials_expiration' value in domain": {
			aadConfigPath: "aad-missing_expiration-domain.conf",
		},
		"aad.conf with missing required 'tenant_id' value in domain": {
			aadConfigPath: "aad-missing_tenantId-domain.conf",
		},
		"aad.conf with missing required 'app_id' value in domain": {
			aadConfigPath: "aad-missing_appId-domain.conf",
		},

		// Missing values in file
		"aad.conf with missing 'homedirpattern'": {
			aadConfigPath: "aad-missing_homedirpattern.conf",
		},
		"aad.conf with missing 'shell'": {
			aadConfigPath: "aad-missing_shell.conf",
		},
		"aad.conf with missing 'homedirpattern' and 'shell'": {
			aadConfigPath: "aad-missing_homedirpattern_and_shell.conf",
		},
		"aad.conf with missing 'offline_credentials_expiration'": {
			aadConfigPath: "aad-missing_expiration.conf",
		},

		// Values only in domain
		"aad.conf with 'homedirpattern' only in domain": {
			aadConfigPath: "aad-homedirpattern_only_in_domain.conf",
		},
		"add.conf with 'shell' only in domain": {
			aadConfigPath: "aad-shell_only_in_domain.conf",
		},
		"aad.conf with 'homedirpattern' and 'shell' only in domain": {
			aadConfigPath: "aad-homedirpattern_and_shell_only_in_domain.conf",
		},
		"aad.conf with 'offline_credentials_expiration' only in domain": {
			aadConfigPath: "aad-expiration_only_in_domain.conf",
		},
		"aad.conf with 'tenant_id' only in domain": {
			aadConfigPath: "aad-tenantId_only_in_domain.conf",
		},
		"aad.conf with 'app_id' only in domain": {
			aadConfigPath: "aad-appId_only_in_domain.conf",
		},

		// Special Cases
		"aad.conf with missing 'homedir' and 'shell' values and wrong adduser.conf": {
			aadConfigPath: "aad-missing_homedirpattern_and_shell.conf",
			addUserPath:   "doesnotexist.conf",
		},
		"aad.conf with invalid 'offline_credentials_expiration' value": {
			aadConfigPath: "aad-invalid_expiration.conf",
		},
		"aad.conf with invalid 'offline_credentials_expiration' value in domain": {
			aadConfigPath: "aad-invalid_expiration-domain.conf",
		},

		// Error cases
		"aad.conf does not exist": {
			aadConfigPath: "doestnotexists.conf",
			wantErr:       true,
		},
		"aad.conf missing 'tenant_id' value": {
			aadConfigPath: "aad-missing_tenantId.conf",
			wantErr:       true,
		},
		"aad.conf missing 'app_id' value": {
			aadConfigPath: "aad-missing_appId.conf",
			wantErr:       true,
		},
	}

	for name, tc := range tests {
		def := strings.ToLower(strings.ReplaceAll(name, " ", "_"))
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tc.aadConfigPath = filepath.Join(testFilesPath, tc.aadConfigPath)

			domain := "domain.com"
			if tc.domain != "" {
				domain = tc.domain
			}
			cfg, err := config.Load(context.Background(), tc.aadConfigPath, domain, config.WithCustomConfPath(tc.addUserPath))
			if tc.wantErr {
				require.Error(t, err, "LoadConfig should have failed, but didn't")
				return
			}

			goldenPath := filepath.Join(testFilesPath, "golden", def)
			wantConfig := testutils.SaveAndLoadFromGolden(t, cfg, testutils.WithCustomGoldPath(goldenPath))
			require.Equal(t, wantConfig, cfg, "Got config and expected config are different")
		})
	}

}

func TestMain(m *testing.M) {
	testutils.InstallUpdateFlag()
	m.Run()
}
