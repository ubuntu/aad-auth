package main

import (
	"context"
	"errors"
)

var (
	pamSystemErr = errors.New("PAM SYSTEM ERROR")
	pamAuthErr   = errors.New("PAM AUTH ERROR")
	pamIgnore    = errors.New("PAM IGNORE")
)

//export pam_sm_authenticate
func authenticate(ctx context.Context, conf string) error {
	// Load configuration.
	tenantID, appID, err := tenantAndAppIDFromConfig(ctx, conf)
	if err != nil {
		pamLogErr(ctx, "No valid configuration found: %v", err)
		return pamSystemErr
	}

	// Get connection information
	username, err := getUser(ctx)
	if err != nil {
		pamLogErr(ctx, "Could not get user from stdin")
		return pamSystemErr
	}
	password, err := getPassword(ctx)
	if err != nil {
		pamLogErr(ctx, "Could not read password from stdin")
		return pamSystemErr
	}

	// AAD authentication
	if err := authenticateAAD(ctx, tenantID, appID, username, password); errors.Is(err, noNetworkErr) {
		return pamIgnore
	} else if errors.Is(err, denyErr) {
		return pamAuthErr
	} else if err != nil {
		pamLogWarn(ctx, "Unhandled error of type: %v. Denying access.", err)
		return pamAuthErr
	}

	// Successful online login, update cache
	c, err := NewCache(ctx)
	if err != nil {
		pamLogErr(ctx, "%v. Denying access.", err)
		return pamAuthErr
	}

	if err := c.Update(ctx, username, password); err != nil {
		pamLogErr(ctx, "%v. Denying access.", err)
		return pamAuthErr
	}

	return nil
}

func main() {}
