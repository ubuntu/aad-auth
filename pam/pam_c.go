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

	"github.com/ubuntu/aad-auth/internal/pam"
)

const (
	defaultConfigPath = "/etc/aad.conf"
)

//go:generate go build -buildmode=c-shared -o pam_aad.so

//export pam_sm_authenticate
func pam_sm_authenticate(pamh *C.pam_handle_t, flags, argc C.int, argv **C.char) C.int {

	ctx := pam.CtxWithPamh(context.Background(), pam.Handle(pamh))

	// Get options.
	conf := defaultConfigPath
	for _, arg := range sliceFromArgv(argc, argv) {
		opt := strings.Split(arg, "=")
		switch opt[0] {
		case "conf":
			conf = opt[1]
		default:
			pam.LogWarn(ctx, "unknown option: %s\n", opt[0])
		}
	}

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
