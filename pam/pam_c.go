package main

/*
#cgo LDFLAGS: -lpam -fPIC
#include <security/pam_appl.h>
#include <security/pam_ext.h>
#include <stdlib.h>
#include <string.h>

char *get_user(pam_handle_t *pamh);
char *get_password(pam_handle_t *pamh);
char *string_from_argv(int i, char **argv);
*/
import "C"
import (
	"context"
	"strings"

	"github.com/ubuntu/aad-auth/internal/logger"
	"github.com/ubuntu/aad-auth/internal/pam"
)

const (
	defaultConfigPath = "/etc/aad.conf"
)

//go:generate go build -ldflags="-s -w" -buildmode=c-shared -o pam_aad.so

//export pam_sm_authenticate
func pam_sm_authenticate(pamh *C.pam_handle_t, flags, argc C.int, argv **C.char) C.int {

	// Attach logger and info handler.
	ctx := pam.CtxWithPamh(context.Background(), pam.Handle(pamh))
	pamLogger := pam.NewLogger(pam.Handle(pamh), pam.LOG_INFO)

	// Get options.
	conf := defaultConfigPath
	for _, arg := range sliceFromArgv(argc, argv) {
		opt, optarg, _ := strings.Cut(arg, "=")
		switch opt {
		case "conf":
			conf = optarg
		case "debug":
			pamLogger = pam.NewLogger(pam.Handle(pamh), pam.LOG_DEBUG)
			pamLogger.Debug("PAM AAD DEBUG enabled")
		default:
			pamLogger.Warn("unknown option: %s\n", opt[0])
		}
	}
	ctx = logger.CtxWithLogger(ctx, pamLogger)

	if err := authenticate(ctx, conf); err != nil {
		switch err {
		case pamSystemErr:
			return C.PAM_SYSTEM_ERR
		case pamAuthErr:
			return C.PAM_AUTH_ERR
		case pamIgnore:
			return C.PAM_IGNORE
		}
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
