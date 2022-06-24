package main

/*
#cgo LDFLAGS: -lpam -fPIC
#include <security/pam_appl.h>
#include <security/pam_ext.h>
#include <stdlib.h>
*/
import "C"
import (
	"context"
	"errors"
	"strings"
)

const (
	defaultConfigPath = "/etc/aad.conf"
)

//go:generate go build -buildmode=c-shared -o pam_aad.so

//export pam_sm_authenticate
func pam_sm_authenticate(pamh *C.pam_handle_t, flags, argc C.int, argv **C.char) C.int {

	ctx := context.WithValue(context.Background(), pamhCtxKey, pamh)

	// Get options.
	conf := defaultConfigPath
	for _, arg := range sliceFromArgv(argc, argv) {
		opt := strings.Split(arg, "=")
		switch opt[0] {
		case "conf":
			conf = opt[1]
		default:
			pamLogWarn(ctx, "unknown option: %s\n", opt[0])
		}
	}

	// Load configuration.
	tenantID, appID, err := tenantAndAppIDFromConfig(ctx, conf)
	if err != nil {
		pamLogErr(ctx, "No valid configuration found: %v", err)
		return C.PAM_SYSTEM_ERR
	}

	// Get connection information
	username, err := getUser(ctx)
	if err != nil {
		pamLogErr(ctx, "Could not get user from stdin")
		return C.PAM_SYSTEM_ERR
	}
	password, err := getPassword(ctx)
	if err != nil {
		pamLogErr(ctx, "Could not read password from stdin")
		return C.PAM_SYSTEM_ERR
	}

	// AAD authentication
	if err := authenticateAAD(ctx, tenantID, appID, username, password); errors.Is(err, noNetworkErr) {
		return C.PAM_IGNORE
	} else if errors.Is(err, pamDenyErr) {
		return C.PAM_AUTH_ERR
	} else if err != nil {
		pamLogWarn(ctx, "Unhandled error of type: %v. Denying access.", err)
		return C.PAM_AUTH_ERR
	}

	// Successful online login, update cache
	c, err := NewCache(ctx)
	if err != nil {
		pamLogErr(ctx, "%v. Denying access.", err)
		return C.PAM_AUTH_ERR
	}

	if err := c.Update(ctx, username, password); err != nil {
		pamLogErr(ctx, "%v. Denying access.", err)
		return C.PAM_AUTH_ERR
	}

	return C.PAM_SUCCESS
}

//export pam_sm_setcred
func pam_sm_setcred(pamh *C.pam_handle_t, flags, argc C.int, argv **C.char) C.int {
	return C.PAM_IGNORE
}

//export pam_sm_open_session
func pam_sm_open_session(pamh *C.pam_handle_t, flags, argc C.int, argv **C.char) C.int {
	return C.PAM_SUCCESS
}

//export pam_sm_close_session
func pam_sm_close_session(pamh *C.pam_handle_t, flags, argc C.int, argv **C.char) C.int {
	return C.PAM_SUCCESS
}

func main() {}
