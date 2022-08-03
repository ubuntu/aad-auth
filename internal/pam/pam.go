package pam

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ubuntu/aad-auth/internal/aad"
	"github.com/ubuntu/aad-auth/internal/cache"
	"github.com/ubuntu/aad-auth/internal/config"
	"github.com/ubuntu/aad-auth/internal/logger"
	"github.com/ubuntu/aad-auth/internal/user"
)

var (
	// ErrPamSystem represents a PAM system error.
	ErrPamSystem = errors.New("PAM SYSTEM ERROR")
	// ErrPamAuth represents a PAM auth error.
	ErrPamAuth = errors.New("PAM AUTH ERROR")
	// ErrPamIgnore represents a PAM ignore return code.
	ErrPamIgnore = errors.New("PAM IGNORE")
)

// Authenticate tries to authenticate user with AAD.
// It’s using the given aad configuration files to get tenant and client appid.
func Authenticate(ctx context.Context, conf string) error {
	// Get connection information
	username, err := getUser(ctx)
	if err != nil {
		logError(ctx, "Could not get user from stdin", nil)
		return ErrPamSystem
	}
	username = user.NormalizeName(username)
	password, err := getPassword(ctx)
	if err != nil {
		logError(ctx, "Could not read password from stdin", nil)
		return ErrPamSystem
	}

	// Load configuration.
	_, domain, _ := strings.Cut(username, "@")
	cfg, err := config.Load(ctx, conf, domain)
	if err != nil {
		logger.Err(ctx, "No valid configuration found: %v", err)
		return ErrPamSystem
	}

	// AAD authentication
	errAAD := aad.Authenticate(ctx, cfg.TenantID, cfg.AppID, username, password)
	if errors.Is(errAAD, aad.ErrDeny) {
		return ErrPamAuth
	} else if errAAD != nil && !errors.Is(errAAD, aad.ErrNoNetwork) {
		logger.Warn(ctx, "Unhandled error of type: %v. Denying access.", errAAD)
		return ErrPamAuth
	}

	c, err := cache.New(ctx, cache.WithOfflineCredentialsExpiration(cfg.OfflineCredentialsExpiration))
	if err != nil {
		logger.Err(ctx, "%v. Denying access.", err)
		return ErrPamAuth
	}
	defer c.Close(ctx)

	// No network: try validate user from cache.
	if errors.Is(errAAD, aad.ErrNoNetwork) {
		if err := c.CanAuthenticate(ctx, username, password); err != nil {
			if errors.Is(err, cache.ErrOfflineCredentialsExpired) {
				Info(ctx, "Machine is offline and cached credentials expired. Please try again when the machine is online.")
			}
			logError(ctx, "%w. Denying access.", err)
			return ErrPamAuth
		}
		return nil
	}

	// Successful online login, update cache.
	if err := c.Update(ctx, username, password, cfg.HomeDirPattern, cfg.Shell); err != nil {
		logError(ctx, "%w. Denying access.", err)
		return ErrPamAuth
	}

	return nil
}

func logError(ctx context.Context, format string, err error) {
	msg := format
	if err != nil {
		msg = fmt.Errorf(format, err).Error()
	}
	logger.Err(ctx, msg)
}
