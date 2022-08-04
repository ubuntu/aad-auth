//go:build !integrationtests

package main

import "github.com/ubuntu/aad-auth/internal/pam"

// supportedOption has no non dealt supported option in production
func supportedOption(pamLogger *pam.Logger, opt, arg string) bool {
	return false
}
