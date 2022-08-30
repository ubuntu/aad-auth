package main

/*
#include <security/pam_appl.h>
#include <stdlib.h>
#include <string.h>

char *string_from_argv(int i, char **argv) {
  return strdup(argv[i]);
}

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

  // pam_get_item will still return PAM_SUCCESS on Ctrl+C when asking for a password.
  if (passwd == NULL) {
    return NULL;
  }
  return strdup(passwd);
}
*/
import "C"
import (
	"fmt"
	"unsafe"

	"github.com/ubuntu/aad-auth/internal/i18n"
)

// pamHandle allows to pass C.pam_handle_t to this package.
type pamHandle = *C.pam_handle_t

func sliceFromArgv(argc C.int, argv **C.char) []string {
	r := make([]string, 0, argc)
	for i := 0; i < int(argc); i++ {
		s := C.string_from_argv(C.int(i), argv)
		defer C.free(unsafe.Pointer(s))
		r = append(r, C.GoString(s))
	}
	return r
}

func getUser(pamh *C.pam_handle_t) (string, error) {
	cUsername := C.get_user(pamh)
	if cUsername == nil {
		return "", fmt.Errorf(i18n.G("no user found"))
	}
	defer C.free(unsafe.Pointer(cUsername))
	return C.GoString(cUsername), nil
}

func getPassword(pamh *C.pam_handle_t) (string, error) {
	cPasswd := C.get_password(pamh)
	if cPasswd == nil {
		return "", fmt.Errorf(i18n.G("no password found"))
	}
	defer C.free(unsafe.Pointer(cPasswd))
	return C.GoString(cPasswd), nil
}
