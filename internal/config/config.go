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

	defaultHomePattern = "/home/%f"
	defaultShell       = "/bin/bash"
)

// AAD represents the configuration values that are used for AAD.
type AAD struct {
	TenantID                     string
	AppID                        string
	OfflineCredentialsExpiration *int
	HomeDirPattern               string
	Shell                        string
}

type options struct {
	addUserConfPath string
}

// Option represents the functional option passed to LoadDefaults.
type Option func(*options)

// Load returns the loaded configuration of the specified domain from p.
// If there is no section for the specified domain, the values on the beginning of p are used as default.
// Should some required values not exist, an error is returned.
func Load(ctx context.Context, p, domain string, opts ...Option) (config AAD, err error) {
	// adding more info to the error message
	defer func() {
		if err != nil {
			err = fmt.Errorf("could not load valid configuration from %s: %v", p, err)
		}
	}()
	logger.Debug(ctx, "Loading configuration from %s", p)

	o := options{
		addUserConfPath: adduserConfPath,
	}
	// applies options
	for _, opt := range opts {
		opt(&o)
	}

	config = AAD{
		HomeDirPattern: defaultHomePattern,
		Shell:          defaultShell,
	}

	// Tries to load the defaults from the adduser.conf
	dh, ds := loadDefaultHomeAndShell(ctx, o.addUserConfPath)
	if dh != "" {
		config.HomeDirPattern = dh
	}
	if ds != "" {
		config.Shell = ds
	}

	cfg, err := ini.Load(p)
	if err != nil {
		return AAD{}, fmt.Errorf("could not open file %s: %v", p, err)
	}

	// Load default section first, and then override with domain specified keys.
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
				config.OfflineCredentialsExpiration = nil
			} else {
				config.OfflineCredentialsExpiration = &v
			}
		}
		if tmp := cfgSection.Key("homedir").String(); tmp != "" {
			config.HomeDirPattern = tmp
		}
		if tmp := cfgSection.Key("shell").String(); tmp != "" {
			config.Shell = tmp
		}
	}

	if config.TenantID == "" {
		return AAD{}, fmt.Errorf("missing required 'tenant_id' entry in configuration file")
	}
	if config.AppID == "" {
		return AAD{}, fmt.Errorf("missing required 'app_id' entry in configuration file")
	}

	return config, nil
}

// loadDefaultHomeAndShell returns default home and shell patterns for all users.
// They will load from an adduser.conf formatted ini file.
// In case they are commented or not defined, we will use hardcoded defaults.
func loadDefaultHomeAndShell(ctx context.Context, path string) (home, shell string) {
	if path == "" {
		return "", ""
	}

	var dh, ds string
	conf, err := ini.Load(path)
	if err != nil {
		logger.Debug(ctx, "Could not open %s, using defaults for homedir and shell: %v", path, err)
		return dh, ds
	}

	if tmp := conf.Section("").Key("DHOME").String(); tmp != "" {
		// DHOME is only the base home directory for all users.
		dh = filepath.Join(tmp, "%f")
	}
	ds = conf.Section("").Key("DSHELL").String()

	return dh, ds
}
