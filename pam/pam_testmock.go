//go:build integrationtests

// This tag is only used for integration tests. It allows to control the mocks only with the build
// configuration via local values.
// The net advantage is that this code, which is poking security holes as we can have the PAM module
// not running as root and save the cache in temporary directory is never shipped in production.

package main

import (
	"strconv"

	"github.com/ubuntu/aad-auth/internal/aad"
	"github.com/ubuntu/aad-auth/internal/cache"
	"github.com/ubuntu/aad-auth/internal/pam"
)

func init() {
	// we want to log on stderr instead of syslog when running integration tests
	logsOnStderr = true
}

// supportedOption allows to add or reset more options to to control the pam module
// cache and pam behaviour.
func supportedOption(pamLogger *pam.Logger, opt, arg string) bool {
	var newOpt pam.Option
	switch opt {
	case "reset":
		opts = nil
		return true
	case "rootUID":
		v, err := strconv.Atoi(arg)
		if err != nil {
			pamLogger.Warn("%v for %s parameter is not an int", v)
			return false
		}
		newOpt = pam.WithCacheOptions([]cache.Option{cache.WithRootUID(v)})
	case "rootGID":
		v, err := strconv.Atoi(arg)
		if err != nil {
			pamLogger.Warn("%v for %s parameter is not an int", v)
			return false
		}
		newOpt = pam.WithCacheOptions([]cache.Option{cache.WithRootGID(v)})
	case "shadowGID":
		v, err := strconv.Atoi(arg)
		if err != nil {
			pamLogger.Warn("%v for %s parameter is not an int", v)
			return false
		}
		newOpt = pam.WithCacheOptions([]cache.Option{cache.WithShadowGID(v)})
	case "cachedir":
		newOpt = pam.WithCacheOptions([]cache.Option{cache.WithCacheDir(arg)})
	case "mockaad":
		newOpt = pam.WithAuthenticator(aad.NewWithMockClient())
	default:
		return false
	}

	opts = append(opts, newOpt)
	return true
}
