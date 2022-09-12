// Package pam is the package which is pure Go code behaving as a pam module.
package pam

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ubuntu/aad-auth/internal/aad"
	"github.com/ubuntu/aad-auth/internal/cache"
	"github.com/ubuntu/aad-auth/internal/config"
	"github.com/ubuntu/aad-auth/internal/i18n"
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

// Authenticator is a interface that wraps the Authenticate method.
type Authenticator interface {
	Authenticate(ctx context.Context, cfg config.AAD, username, password string) error
}

type option struct {
	auth      Authenticator
	cacheOpts []cache.Option
}

// Option allows to change Authenticate for mocking in tests.
type Option func(*option)

// WithAuthenticator overrides the default authenticator.
func WithAuthenticator(auth Authenticator) Option {
	return func(o *option) {
		o.auth = auth
	}
}

// WithCacheOptions overrides append additional cache options.
func WithCacheOptions(cacheOpts []cache.Option) Option {
	return func(o *option) {
		o.cacheOpts = append(o.cacheOpts, cacheOpts...)
	}
}

// Authenticate tries to authenticate user with the given Authenticater.
// It’s passing specific configuration, per domain, so that that Authenticater can use them.
func Authenticate(ctx context.Context, username, password, conf string, opts ...Option) error {
	username = user.NormalizeName(username)

	// Load configuration.
	_, domain, _ := strings.Cut(username, "@")
	cfg, err := config.Load(ctx, conf, domain)
	if err != nil {
		logger.Err(ctx, i18n.G("No valid configuration found: %v"), err)
		return ErrPamSystem
	}

	// Apply options and config
	o := option{
		auth: aad.AAD{},
	}
	if cfg.OfflineCredentialsExpiration != nil {
		o.cacheOpts = append(o.cacheOpts, cache.WithOfflineCredentialsExpiration(*cfg.OfflineCredentialsExpiration))
	}
	for _, opt := range opts {
		opt(&o)
	}

	// Authentication. Note that the errors are AAD errors for now, but we can decorelate them in the future.
	errAAD := o.auth.Authenticate(ctx, cfg, username, password)
	if errors.Is(errAAD, aad.ErrDeny) {
		return ErrPamAuth
	} else if errAAD != nil && !errors.Is(errAAD, aad.ErrNoNetwork) {
		logger.Warn(ctx, i18n.G("Unhandled error of type: %v. Denying access."), errAAD)
		return ErrPamAuth
	}

	c, err := cache.New(ctx, o.cacheOpts...)
	if err != nil {
		logError(ctx, i18n.G("%w. Denying access."), err)
		return ErrPamSystem
	}
	defer c.Close(ctx)

	// No network: try validate user from cache.
	if errors.Is(errAAD, aad.ErrNoNetwork) {
		if err := c.CanAuthenticate(ctx, username, password); err != nil {
			if errors.Is(err, cache.ErrOfflineCredentialsExpired) {
				Info(ctx, i18n.G("Machine is offline and cached credentials expired. Please try again when the machine is online."))
			}
			if errors.Is(err, cache.ErrOfflineAuthDisabled) {
				Info(ctx, i18n.G("Machine is offline and offline authentication is disabled. Please try again when the machine is online."))
			}
			logError(ctx, i18n.G("%w. Denying access."), err)
			return ErrPamAuth
		}
		return nil
	}

	// Successful online login, update cache.
	if err := c.Update(ctx, username, password, cfg.HomeDirPattern, cfg.Shell); err != nil {
		logError(ctx, i18n.G("%w. Denying access."), err)
		return ErrPamAuth
	}

	return nil
}

func logError(ctx context.Context, format string, err error) {
	err = fmt.Errorf(format, err)
	logger.Err(ctx, err.Error())
}
