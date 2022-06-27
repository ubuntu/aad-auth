package main

import (
	"context"
	"errors"

	"github.com/ubuntu/aad-auth/internal/aad"
	"github.com/ubuntu/aad-auth/internal/cache"
	"github.com/ubuntu/aad-auth/internal/pam"
)

var (
	pamSystemErr = errors.New("PAM SYSTEM ERROR")
	pamAuthErr   = errors.New("PAM AUTH ERROR")
	pamIgnore    = errors.New("PAM IGNORE")
)

//export pam_sm_authenticate
func authenticate(ctx context.Context, conf string) error {
	// Load configuration.
	tenantID, appID, err := loadConfig(ctx, conf)
	if err != nil {
		pam.LogErr(ctx, "No valid configuration found: %v", err)
		return pamSystemErr
	}

	// Get connection information
	username, err := pam.GetUser(ctx)
	if err != nil {
		pam.LogErr(ctx, "Could not get user from stdin")
		return pamSystemErr
	}
	password, err := pam.GetPassword(ctx)
	if err != nil {
		pam.LogErr(ctx, "Could not read password from stdin")
		return pamSystemErr
	}

	// AAD authentication
	errAAD := aad.Authenticate(ctx, tenantID, appID, username, password)
	if errors.Is(errAAD, aad.DenyErr) {
		return pamAuthErr
	} else if errAAD != nil && !errors.Is(errAAD, aad.NoNetworkErr) {
		pam.LogWarn(ctx, "Unhandled error of type: %v. Denying access.", errAAD)
		return pamAuthErr
	}

	c, err := cache.New(ctx)
	if err != nil {
		pam.LogErr(ctx, "%v. Denying access.", err)
		return pamAuthErr
	}
	defer c.Close()

	// No network: try validate user from cache.
	if errors.Is(errAAD, aad.NoNetworkErr) {
		if err := c.CanAuthenticate(ctx, username, password); err != nil {
			pam.LogErr(ctx, "%v. Denying access.", err)
			return pamAuthErr
		}
		return nil
	}

	// Successful online login, update cache.
	if err := c.Update(ctx, username, password); err != nil {
		pam.LogErr(ctx, "%v. Denying access.", err)
		return pamAuthErr
	}

	return nil
}

func main() {}
