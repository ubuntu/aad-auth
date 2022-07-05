package main

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-ini/ini"
	"github.com/ubuntu/aad-auth/internal/logger"
)

// loadConfig returns the loaded configuration from p.
func loadConfig(ctx context.Context, p string) (tenantID string, appID string, cacheRevalidation int, err error) {
	logger.Debug(ctx, "Loading configuration from %s", p)

	cfg, err := ini.Load(p)
	if err != nil {
		return "", "", 0, fmt.Errorf("loading configuration failed: %v", err)
	}

	tenantID = cfg.Section("").Key("tenant_id").String()
	appID = cfg.Section("").Key("app_id").String()
	cacheRevalidation = -1
	cacheRevalidationCfg := cfg.Section("").Key("cache_revalidation").String()
	if cacheRevalidationCfg != "" {
		v, err := strconv.Atoi(cacheRevalidationCfg)
		if err != nil {
			logger.Warn(ctx, "Invalid cache revalidation period %v", err)
		}
		cacheRevalidation = v
	}

	if tenantID == "" {
		return "", "", 0, fmt.Errorf("missing 'tenant_id' entry in configuration file")
	}
	if appID == "" {
		return "", "", 0, fmt.Errorf("missing 'app_id' entry in configuration file")
	}

	return tenantID, appID, cacheRevalidation, nil
}
