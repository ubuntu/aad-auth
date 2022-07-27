package main

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

// loadConfig returns the loaded configuration of the specified domain from p.
// If there is no section for the specified domain, the values on the beginning of p are used as default.
func loadConfig(ctx context.Context, p, domain string) (tenantID string, appID string, offlineCredentialsExpiration int, homeDir string, shell string, err error) {
	logger.Debug(ctx, "Loading configuration from %s", p)

	cfg, err := ini.Load(p)
	if err != nil {
		return "", "", 0, "", "", fmt.Errorf("loading configuration failed: %v", err)
	}

	domainSection := cfg.Section("")
	if cfg.HasSection(domain) {
		domainSection = cfg.Section(domain)
	}

	tenantID = domainSection.Key("tenant_id").String()
	appID = domainSection.Key("app_id").String()
	offlineCredentialsExpiration = -1
	homeDir = domainSection.Key("homedir").String()
	shell = domainSection.Key("shell").String()

	cacheRevalidationCfg := domainSection.Key("offline_credentials_expiration").String()
	if cacheRevalidationCfg != "" {
		v, err := strconv.Atoi(cacheRevalidationCfg)
		if err != nil {
			logger.Warn(ctx, "Invalid cache revalidation period %v", err)
		}
		offlineCredentialsExpiration = v
	}

	if tenantID == "" {
		return "", "", 0, "", "", fmt.Errorf("missing 'tenant_id' entry in configuration file")
	}
	if appID == "" {
		return "", "", 0, "", "", fmt.Errorf("missing 'app_id' entry in configuration file")
	}

	// Tries to fall back to the aad.conf default homedir before looking into adduser.conf
	if homeDir == "" && domainSection.Name() != "" {
		homeDir = cfg.Section("").Key("homedir").String()
	}
	// Tries to fall back to the aad.conf default shell before looking into adduser.conf
	if shell == "" && domainSection.Name() != "" {
		shell = cfg.Section("").Key("shell").String()
	}

	// Only open the config file once, if required.
	if homeDir == "" || shell == "" {
		dh, ds := loadDefaultHomeAndShell(ctx, adduserConfPath)
		if homeDir == "" {
			homeDir = dh
		}
		if shell == "" {
			shell = ds
		}
	}

	return tenantID, appID, offlineCredentialsExpiration, homeDir, shell, nil
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
