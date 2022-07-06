package main

import (
	"context"
	"errors"

	"github.com/ubuntu/aad-auth/internal/aad"
	"github.com/ubuntu/aad-auth/internal/cache"
	"github.com/ubuntu/aad-auth/internal/logger"
	"github.com/ubuntu/aad-auth/internal/pam"
)

var (
	// ErrPamSystem represents a PAM system error.
	ErrPamSystem = errors.New("PAM SYSTEM ERROR")
	// ErrPamAuth represents a PAM auth error.
	ErrPamAuth = errors.New("PAM AUTH ERROR")
	// ErrPamIgnore represents a PAM ignore return code.
	ErrPamIgnore = errors.New("PAM IGNORE")
)

//export pam_sm_authenticate
func authenticate(ctx context.Context, conf string) error {
	// Load configuration.
	tenantID, appID, offlineCredentialsExpiration, err := loadConfig(ctx, conf)
	if err != nil {
		logger.Err(ctx, "No valid configuration found: %v", err)
		return ErrPamSystem
	}

	// Get connection information
	username, err := pam.GetUser(ctx)
	if err != nil {
		logger.Err(ctx, "Could not get user from stdin")
		return ErrPamSystem
	}
	password, err := pam.GetPassword(ctx)
	if err != nil {
		logger.Err(ctx, "Could not read password from stdin")
		return ErrPamSystem
	}

	// AAD authentication
	errAAD := aad.Authenticate(ctx, tenantID, appID, username, password)
	if errors.Is(errAAD, aad.ErrDeny) {
		return ErrPamAuth
	} else if errAAD != nil && !errors.Is(errAAD, aad.ErrNoNetwork) {
		logger.Warn(ctx, "Unhandled error of type: %v. Denying access.", errAAD)
		return ErrPamAuth
	}

	c, err := cache.New(ctx, cache.WithOfflineCredentialsExpiration(offlineCredentialsExpiration))
	if err != nil {
		logger.Err(ctx, "%v. Denying access.", err)
		return ErrPamAuth
	}
	defer c.Close()

	// No network: try validate user from cache.
	if errors.Is(errAAD, aad.ErrNoNetwork) {
		if err := c.CanAuthenticate(ctx, username, password); err != nil {
			logger.Err(ctx, "%v. Denying access.", err)
			return ErrPamAuth
		}
		return nil
	}

	// Successful online login, update cache.
	if err := c.Update(ctx, username, password); err != nil {
		logger.Err(ctx, "%v. Denying access.", err)
		return ErrPamAuth
	}

	return nil
}

func main() {}
