//go:build !integrationtests

package main

import "github.com/ubuntu/aad-auth/internal/pam"

// supportedOption does nothing in production: all supported options are in main code. It is for integration tests only.
func supportedOption(_ *pam.Logger, _, _ string) bool {
	return false
}
