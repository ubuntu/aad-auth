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

type Authenticater interface {
	Authenticate(ctx context.Context, cfg config.AAD, username, password string) error
}

// Authenticate tries to authenticate user with the given Authenticater.
// It’s passing specific configuration, per domain, so that that Authenticater can use them.
func Authenticate(ctx context.Context, auth Authenticater, conf string) error {
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

	// Authentication. Note that the errors are AAD errors for now, but we can decorelate them in the future.
	errAAD := auth.Authenticate(ctx, cfg, username, password)
	if errors.Is(errAAD, aad.ErrDeny) {
		return ErrPamAuth
	} else if errAAD != nil && !errors.Is(errAAD, aad.ErrNoNetwork) {
		logger.Warn(ctx, "Unhandled error of type: %v. Denying access.", errAAD)
		return ErrPamAuth
	}

	opts := []cache.Option{}
	if cfg.OfflineCredentialsExpiration != nil {
		opts = append(opts, cache.WithOfflineCredentialsExpiration(*cfg.OfflineCredentialsExpiration))
	}
	c, err := cache.New(ctx, opts...)
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
