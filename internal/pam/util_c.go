package pam

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
*/
import "C"
import (
	"context"
	"errors"
	"fmt"
	"unsafe"
)

func getUser(ctx context.Context) (string, error) {
	pamh, ok := ctx.Value(ctxPamhKey).(*C.pam_handle_t)
	if !ok {
		return "", errors.New("can't check for user: no pam context")
	}

	cUsername := C.get_user(pamh)
	if cUsername == nil {
		return "", fmt.Errorf("no user found")
	}
	defer C.free(unsafe.Pointer(cUsername))
	return C.GoString(cUsername), nil
}

func getPassword(ctx context.Context) (string, error) {
	pamh, ok := ctx.Value(ctxPamhKey).(*C.pam_handle_t)
	if !ok {
		return "", errors.New("can't check for user: no pam context")
	}

	cPasswd := C.get_password(pamh)
	if cPasswd == nil {
		return "", fmt.Errorf("no password found")
	}
	defer C.free(unsafe.Pointer(cPasswd))
	return C.GoString(cPasswd), nil
}
