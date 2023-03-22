// Package config is the package dealing with aad-auth configuration files.
package config

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/go-ini/ini"
	"github.com/ubuntu/aad-auth/internal/i18n"
	"github.com/ubuntu/aad-auth/internal/logger"
	"github.com/ubuntu/decorate"
)

const (
	adduserConfPath = "/etc/adduser.conf"

	defaultHomePattern = "/home/%f"
	defaultShell       = "/bin/bash"
)

// AAD represents the configuration values that are used for AAD.
type AAD struct {
	TenantID                     string `ini:"tenant_id"`
	AppID                        string `ini:"app_id"`
	OfflineCredentialsExpiration *int   `ini:"offline_credentials_expiration"`
	HomeDirPattern               string `ini:"homedir"`
	Shell                        string `ini:"shell"`
}

// ToIni reflects the configuration values to an ini.File representation.
func (a AAD) ToIni() (*ini.File, error) {
	cfg := ini.Empty()
	if err := ini.ReflectFrom(cfg, &a); err != nil {
		return nil, fmt.Errorf("could not reflect configuration to ini.File: %w", err)
	}

	return cfg, nil
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
	defer decorate.OnError(&err, i18n.G("could not load valid configuration from %s"), p)

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
		return AAD{}, fmt.Errorf("could not open file %s: %w", p, err)
	}

	// Load default section first, and then override with domain specified keys.
	for _, section := range []string{ini.DefaultSection, domain} {
		if err := cfg.Section(section).StrictMapTo(&config); err != nil {
			return AAD{}, err
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

// Validate validates a given configuration file.
func Validate(ctx context.Context, p string) error {
	cfg, err := ini.Load(p)
	if err != nil {
		return err
	}

	// Config sections are domains, so check them all if present
	for _, domain := range cfg.SectionStrings() {
		// Skip default section if we have multiple domains, as users might set
		// required options only in the domain sections
		if domain == ini.DefaultSection && len(cfg.Sections()) > 1 {
			continue
		}
		if _, err = Load(ctx, p, domain); err != nil {
			return err
		}
	}
	return nil
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
