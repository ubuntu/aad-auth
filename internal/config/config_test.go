package config_test

import (
	"context"
	"flag"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-ini/ini"
	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/config"
	"github.com/ubuntu/aad-auth/internal/testutils"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()
	testFilesPath := filepath.Join("testdata", "TestLoadConfig")

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
		"aad.conf with missing 'homedir' and 'shell' values, but valid adduser.conf": {
			aadConfigPath: "aad-missing_homedirpattern_and_shell.conf",
			addUserPath:   "valid_adduser.conf",
		},
		"aad.conf with missing 'homedir' and 'shell' values and wrong adduser.conf": {
			aadConfigPath: "aad-missing_homedirpattern_and_shell.conf",
			addUserPath:   "doesnotexist.conf",
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
		"aad.conf with invalid 'offline_credentials_expiration' value": {
			aadConfigPath: "aad-invalid_expiration.conf",
			wantErr:       true,
		},
		"aad.conf with invalid 'offline_credentials_expiration' value in domain": {
			aadConfigPath: "aad-invalid_expiration-domain.conf",
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

			got, err := config.Load(context.Background(), tc.aadConfigPath, domain, config.WithAddUserConfPath(tc.addUserPath))
			if tc.wantErr {
				require.Error(t, err, "LoadConfig should have failed, but didn't")
				return
			}
			require.NoError(t, err, "LoadConfig failed when it shouldn't")

			goldenPath := filepath.Join(testFilesPath, "golden", def)
			want := testutils.LoadYAMLWithUpdateFromGolden(t, got, testutils.WithGoldPath(goldenPath))
			require.Equal(t, want, got, "Got config and expected config are different")
		})
	}
}

func TestToIni(t *testing.T) {
	t.Parallel()

	expiration := 90
	aad := config.AAD{TenantID: "tenantID", AppID: "appID", HomeDirPattern: "homeDirPattern", Shell: "shell", OfflineCredentialsExpiration: &expiration}

	want := ini.Empty()
	err := ini.ReflectFrom(want, &aad)
	require.NoError(t, err, "Setup: failed to reflect config to ini")

	got, err := aad.ToIni()
	require.NoError(t, err, "Setup: failed to reflect config to ini")

	require.Equal(t, want, got, "Got and expected ini files are different")
}

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		configFile string
		wantErr    bool
	}{
		"valid config, default domain":   {configFile: "valid.conf"},
		"valid config, multiple domains": {configFile: "valid-multiple-domains.conf"},

		// Error cases
		"invalid config, default domain":   {configFile: "invalid.conf", wantErr: true},
		"invalid config, commented values": {configFile: "invalid-commented.conf", wantErr: true},
		"invalid config, multiple domains": {configFile: "invalid-multiple-domains.conf", wantErr: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			configFile := filepath.Join("testdata", tc.configFile)
			err := config.Validate(context.Background(), configFile)
			if tc.wantErr {
				require.Error(t, err, "Validate should have failed, but didn't")
				return
			}
			require.NoError(t, err, "Validate failed but shouldn't have")
		})
	}
}

func TestMain(m *testing.M) {
	testutils.InstallUpdateFlag()
	flag.Parse()
	m.Run()
}
