package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	msalErrors "github.com/AzureAD/microsoft-authentication-library-for-go/apps/errors"
	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/public"
	"github.com/go-ini/ini"
)

const (
	endpoint = "https://login.microsoftonline.com"

	invalidCredCode = 50126
	requiresMFACode = 50076
	noSuchUserCode  = 50034
)

var (
	noNetworkErr = errors.New("NO NETWORK")
	pamDenyErr   = errors.New("DENY")
	pamSystemErr = errors.New("SYSTEM ERROR")
)

type aadErr struct {
	ErrorCodes []int `json:"error_codes"`
}

func authenticateAAD(ctx context.Context, tenantID, appID, username, password string) error {
	authority := fmt.Sprintf("%s/%s", endpoint, tenantID)
	pamLogDebug(ctx, "Connecting to %q, with clientID %q for user %q", authority, appID, username)

	// Get client from network
	app, errAcquireToken := public.New(appID, public.WithAuthority(authority))
	if errAcquireToken != nil {
		pamLogErr(ctx, "Connection to authority failed: %v", errAcquireToken)
		return noNetworkErr
	}

	// Authentify the user
	_, errAcquireToken = app.AcquireTokenByUsernamePassword(context.Background(), nil, username, password)

	var callErr msalErrors.CallErr
	if errors.As(errAcquireToken, &callErr) {
		data, err := io.ReadAll(callErr.Resp.Body)
		if err != nil {
			pamLogErr(ctx, "Can't read server response: %v", err)
			return pamDenyErr
		}
		var addErrWithCodes aadErr
		if err := json.Unmarshal(data, &addErrWithCodes); err != nil {
			pamLogErr(ctx, "Invalid server response, not a json object: %v", err)
			return pamDenyErr
		}
		for _, errcode := range addErrWithCodes.ErrorCodes {
			if errcode == invalidCredCode {
				pamLogDebug(ctx, "Got response: Invalid credentials")
				return pamDenyErr
			}
			if errcode == noSuchUserCode {
				pamLogDebug(ctx, "Got response: User doesn't exist")
				return pamDenyErr
			}
			if errcode == requiresMFACode {
				pamLogDebug(ctx, "Authentication successful even if requiring MFA")
				return nil
			}
		}
	}

	if errAcquireToken != nil {
		pamLogDebug(ctx, "Unknown error type: %v", errAcquireToken)
		return pamDenyErr
	}

	pamLogDebug(ctx, "Authentication successful with user/password")
	return nil
}

func tenantAndAppIDFromConfig(ctx context.Context, p string) (string, string, error) {
	pamLogDebug(ctx, "Loading configuration from %s", p)

	cfg, err := ini.Load(p)
	if err != nil {
		return "", "", fmt.Errorf("loading configuration failed: %v", err)
	}

	tenantID := cfg.Section("").Key("tenant_id").String()
	appID := cfg.Section("").Key("app_id").String()

	if tenantID == "" {
		return "", "", fmt.Errorf("missing 'tenant_id' entry in configuration file")
	}
	if appID == "" {
		return "", "", fmt.Errorf("missing 'app_id' entry in configuration file")
	}

	return tenantID, appID, nil
}
