package config

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/go-ini/ini"
	"github.com/ubuntu/aad-auth/internal/logger"
)

const (
	adduserConfPath = "/etc/adduser.conf"
)

// AADConfig represents the configuration values that are used for AAD
type AADConfig struct {
	TenantID                     string
	AppID                        string
	OfflineCredentialsExpiration int
	HomeDir                      string
	Shell                        string
}

type options struct {
	addUserConfPath string
}

// Option represents the functional option passed to LoadDefaults
type Option func(*options)

// ! 	 this is a warn, for sure
// ? 	 is this an info?
// TODO: this is a TODO
// * 	 what is this?

//! IF THE TESTS FAIL, COVERAGE ISN'T UPDATED
//* optional functional parameters (Option) for setting adduserConfPath -- only for tests.
//* golden files for tests.

// LoadConfig returns the loaded configuration of the specified domain from p.
// If there is no section for the specified domain, the values on the beginning of p are used as default.
func LoadConfig(ctx context.Context, p, domain string, opts ...Option) (config AADConfig, err error) {
	// adding more info to the error message
	defer func() {
		if err != nil {
			err = fmt.Errorf("could not load valid configuration from %s: %v", p, err)
		}
	}()

	logger.Debug(ctx, "Loading configuration from %s", p)

	cfg, err := ini.Load(p)
	if err != nil {
		return AADConfig{}, fmt.Errorf("could not open file %s: %v", p, err)
	}

	for _, section := range []string{"", domain} {
		cfgSection := cfg.Section(section)
		if tmp := cfgSection.Key("tenant_id").String(); tmp != "" {
			config.TenantID = tmp
		}
		if tmp := cfgSection.Key("app_id").String(); tmp != "" {
			config.AppID = tmp
		}
		if tmp := cfgSection.Key("offline_credentials_expiration").String(); tmp != "" {
			v, err := strconv.Atoi(tmp)
			if err != nil {
				logger.Warn(ctx, "Invalid cache revalidation period %v", err)
			}
			config.OfflineCredentialsExpiration = v
		}
		if tmp := cfgSection.Key("homedir").String(); tmp != "" {
			config.HomeDir = tmp
		}
		if tmp := cfgSection.Key("shell").String(); tmp != "" {
			config.Shell = tmp
		}
	}

	if config.TenantID == "" {
		return AADConfig{}, fmt.Errorf("missing required 'tenant_id' entry in configuration file")
	}
	if config.AppID == "" {
		return AADConfig{}, fmt.Errorf("missing required 'app_id' entry in configuration file")
	}

	o := options{
		addUserConfPath: adduserConfPath,
	}
	// applies options
	for _, opt := range opts {
		opt(&o)
	}

	// Only open the config file once, if required.
	if config.HomeDir == "" || config.Shell == "" {
		dh, ds := loadDefaultHomeAndShell(ctx, o.addUserConfPath)
		if config.HomeDir == "" {
			config.HomeDir = dh
		}
		if config.Shell == "" {
			config.Shell = ds
		}
	}

	return config, nil
}

const (
	defaultHomePattern = "/home/%u"
	defaultShell       = "/bin/bash"
)

// loadDefaultHomeAndShell returns default home and shell patterns for all users.
// They will load from an adduser.conf formatted ini file.
// In case they are commented or not defined, we will use hardcoded defaults.
func loadDefaultHomeAndShell(ctx context.Context, path string) (home, shell string) {
	dh, ds := defaultHomePattern, defaultShell
	conf, err := ini.Load(path)
	if err != nil {
		logger.Debug(ctx, "Could not open %s, using defaults for homedir and shell: %v", path, err)
		return dh, ds
	}

	if tmp := conf.Section("").Key("DHOME").String(); tmp != "" {
		// DHOME is only the base home directory for all users.
		dh = filepath.Join(tmp, "%u")
	}
	if tmp := conf.Section("").Key("DSHELL").String(); tmp != "" {
		ds = tmp
	}
	return dh, ds
}
