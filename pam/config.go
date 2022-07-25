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
	defaultsFile = "/etc/adduser.conf"
)

// loadConfig returns the loaded configuration from p.
func loadConfig(ctx context.Context, p string) (tenantID string, appID string, offlineCredentialsExpiration int, homeDir string, shell string, err error) {
	logger.Debug(ctx, "Loading configuration from %s", p)

	cfg, err := ini.Load(p)
	if err != nil {
		return "", "", 0, "", "", fmt.Errorf("loading configuration failed: %v", err)
	}

	tenantID = cfg.Section("").Key("tenant_id").String()
	appID = cfg.Section("").Key("app_id").String()
	offlineCredentialsExpiration = -1
	homeDir = cfg.Section("").Key("homedir").String()
	shell = cfg.Section("").Key("shell").String()
	cacheRevalidationCfg := cfg.Section("").Key("offline_credentials_expiration").String()
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

	// It's not pretty, but at least it will only open the config file once
	// and only if it's needed
	if homeDir == "" || shell == "" {
		dh, ds := loadDefaultHomeAndShell(ctx, defaultsFile)
		if homeDir == "" {
			homeDir = filepath.Join(dh, "%u")
		}
		if shell == "" {
			shell = ds
		}
	}

	return tenantID, appID, offlineCredentialsExpiration, homeDir, shell, nil
}

func loadDefaultHomeAndShell(ctx context.Context, file string) (string, string) {
	dh, ds := "/home/%u", "/bin/bash"
	conf, err := ini.Load(file)
	if err != nil {
		logger.Debug(ctx, "Could not open %s, using defaults for homedir and shell\n", file)
		return dh, ds
	}

	if tmp := conf.Section("").Key("DHOME").String(); tmp != "" {
		dh = tmp
	}
	if tmp := conf.Section("").Key("DSHELL").String(); tmp != "" {
		ds = tmp
	}
	return dh, ds
}
