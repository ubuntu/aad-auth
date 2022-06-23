package main

/*
#include <security/pam_appl.h>
#include <stdlib.h>
#include <string.h>

char *get_user(pam_handle_t *pamh) {
  if (!pamh)
    return NULL;
  int pam_err = 0;
  const char *user;
  if ((pam_err = pam_get_item(pamh, PAM_USER, (const void**)&user)) != PAM_SUCCESS)
    return NULL;
  return strdup(user);
}

char *get_password(pam_handle_t *pamh) {
  if (!pamh)
    return NULL;
  int pam_err = 0;
  const char *passwd;
  if ((pam_err = pam_get_item(pamh, PAM_AUTHTOK, (const void**)&passwd)) != PAM_SUCCESS)
    return NULL;
  return strdup(passwd);
}

char *string_from_argv(int i, char **argv) {
  return strdup(argv[i]);
}
*/
import "C"
import (
	"context"
	"unsafe"
)

const (
	pamhCtxKey = "pamhCtxKey"
)

func getUser(ctx context.Context) (string, error) {
	pamh := ctx.Value(pamhCtxKey).(*C.pam_handle_t)

	cUsername := C.get_user(pamh)
	if cUsername == nil {
		return "", pamSystemErr
	}
	defer C.free(unsafe.Pointer(cUsername))
	return C.GoString(cUsername), nil
}

func getPassword(ctx context.Context) (string, error) {
	pamh := ctx.Value(pamhCtxKey).(*C.pam_handle_t)

	cPasswd := C.get_password(pamh)
	if cPasswd == nil {
		return "", pamSystemErr
	}
	defer C.free(unsafe.Pointer(cPasswd))
	return C.GoString(cPasswd), nil
}

func sliceFromArgv(argc C.int, argv **C.char) []string {
	r := make([]string, 0, argc)
	for i := 0; i < int(argc); i++ {
		s := C.string_from_argv(C.int(i), argv)
		defer C.free(unsafe.Pointer(s))
		r = append(r, C.GoString(s))
	}
	return r
}
