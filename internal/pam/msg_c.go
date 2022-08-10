package pam

/*
#cgo LDFLAGS: -lpam -fPIC
#include <security/pam_ext.h>
#include <syslog.h>
#include <stdlib.h>

int pam_info_no_variadic(pam_handle_t *pamh, const char *msg) {
	return pam_info(pamh, "%s", msg);
}
*/
import "C"
import (
	"context"
	"fmt"
	"log"
	"unsafe"
)

type pamhCtxKey string

const (
	ctxPamhKey pamhCtxKey = "pamhCtxKey"
)

// CtxWithPamh returns a context with pam handler struct attached.
func CtxWithPamh(ctx context.Context, pamh Handle) context.Context {
	return context.WithValue(ctx, ctxPamhKey, pamh)
}

// Info prints a info message to the pam log.
func Info(ctx context.Context, format string, a ...any) {
	msg := fmt.Sprintf(format, a...)

	pamh, ok := ctx.Value(ctxPamhKey).(*C.pam_handle_t)
	if !ok {
		log.Printf("WARNING: Failed to display message to user (no pam attached): %v\n", msg)
		return
	}

	cMsg := C.CString(msg)
	defer C.free(unsafe.Pointer(cMsg))

	if errInt := C.pam_info_no_variadic(pamh, cMsg); errInt != C.PAM_SUCCESS {
		pamSyslog(pamh, C.LOG_WARNING, "Failed to display message to user (error %d): %v\n", errInt, msg)
		return
	}
}
