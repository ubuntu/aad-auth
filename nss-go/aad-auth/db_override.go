//go:build integrationtests

package main

import (
	"log"
	"os"
	"strconv"

	"github.com/ubuntu/aad-auth/internal/cache"
)

// initialize via env variables in mock test
func init() {
	uidEnv := os.Getenv("NSS_AAD_ROOT_UID")
	if uidEnv != "" {
		uid, err := strconv.Atoi(uidEnv)
		if err != nil {
			log.Fatalf("passed root UID override is not a valid int: %s", err)
		}
		opts = append(opts, cache.WithRootUID(uid))
	}
	gidEnv := os.Getenv("NSS_AAD_ROOT_GID")
	if gidEnv != "" {
		gid, err := strconv.Atoi(gidEnv)
		if err != nil {
			log.Fatalf("passed root GID override is not a valid int: %s", err)
		}
		opts = append(opts, cache.WithRootGID(gid))
	}
	shadowGIDEnv := os.Getenv("NSS_AAD_SHADOW_GID")
	if shadowGIDEnv != "" {
		shadowGID, err := strconv.Atoi(shadowGIDEnv)
		if err != nil {
			log.Fatalf("passed shadow GID override is not a valid int: %s", err)
		}
		opts = append(opts, cache.WithShadowGID(shadowGID))
	}
	if cacheDir := os.Getenv("NSS_AAD_CACHEDIR"); cacheDir != "" {
		opts = append(opts, cache.WithCacheDir(cacheDir))
	}
	if shadowMode := os.Getenv("NSS_AAD_SHADOWMODE"); shadowMode != "" {
		mode, err := strconv.Atoi(shadowMode)
		if err != nil {
			log.Fatalf("passed shadow mode override is not a valid int: %s", err)
		}
		opts = append(opts, cache.WithShadowMode(mode))
	}
}
