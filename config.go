package main

import (
	"context"
	"fmt"

	"github.com/go-ini/ini"
	"github.com/ubuntu/aad-auth/internal/pam"
)

// loadConfig returns the loaded configuration from p.
func loadConfig(ctx context.Context, p string) (tenantID string, appID string, err error) {
	pam.LogDebug(ctx, "Loading configuration from %s", p)

	cfg, err := ini.Load(p)
	if err != nil {
		return "", "", fmt.Errorf("loading configuration failed: %v", err)
	}

	tenantID = cfg.Section("").Key("tenant_id").String()
	appID = cfg.Section("").Key("app_id").String()

	if tenantID == "" {
		return "", "", fmt.Errorf("missing 'tenant_id' entry in configuration file")
	}
	if appID == "" {
		return "", "", fmt.Errorf("missing 'app_id' entry in configuration file")
	}

	return tenantID, appID, nil
}
