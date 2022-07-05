package main

/*
#cgo LDFLAGS: -fPIC
#include <nss.h>
#include <pwd.h>
#include <errno.h>

typedef enum nss_status nss_status;
*/
import "C"
import (
	"context"
	"unsafe"

	"github.com/ubuntu/aad-auth/internal/logger"
	"github.com/ubuntu/aad-auth/internal/nss/passwd"
)

//export _nss_aad_getpwnam_r
func _nss_aad_getpwnam_r(name *C.char, pwd *C.struct_passwd, buf *C.char, buflen C.size_t, errnop *C.int) C.nss_status {
	ctx := ctxWithSyslogLogger(context.Background())
	n := C.GoString(name)
	logger.Debug(ctx, "_nss_aad_getpwnam_r called for %q\n", n)

	p, err := passwd.NewByName(ctx, n)
	if err != nil {
		return errToCStatus(ctx, err, errnop)
	}
	if err = p.ToCpasswd(passwd.CPasswd(unsafe.Pointer(pwd)), (*passwd.CChar)(buf), passwd.CSizeT(buflen)); err != nil {
		return errToCStatus(ctx, err, errnop)
	}

	return C.NSS_STATUS_SUCCESS
}

//export _nss_aad_getpwuid_r
func _nss_aad_getpwuid_r(uid C.uid_t, pwd *C.struct_passwd, buf *C.char, buflen C.size_t, errnop *C.int) C.nss_status {
	ctx := ctxWithSyslogLogger(context.Background())
	logger.Debug(ctx, "_nss_aad_getpwuid_r called for %q\n", uid)

	p, err := passwd.NewByUID(ctx, uint(uid))
	if err != nil {
		return errToCStatus(ctx, err, errnop)
	}
	if err = p.ToCpasswd(passwd.CPasswd(unsafe.Pointer(pwd)), (*passwd.CChar)(buf), passwd.CSizeT(buflen)); err != nil {
		return errToCStatus(ctx, err, errnop)
	}

	return C.NSS_STATUS_SUCCESS
}

//export _nss_aad_setpwent
func _nss_aad_setpwent(stayopen C.int) C.nss_status {
	ctx := ctxWithSyslogLogger(context.Background())
	logger.Debug(ctx, "_nss_aad_setpwent called\n")

	// Initialization of the database is done in the read primitive
	return C.NSS_STATUS_SUCCESS
}

//export _nss_aad_endpwent
func _nss_aad_endpwent() C.nss_status {
	ctx := ctxWithSyslogLogger(context.Background())
	logger.Debug(ctx, "_nss_aad_endpwent called\n")

	// Closing the database is done in the read primitive
	return C.NSS_STATUS_SUCCESS
}

//export _nss_aad_getpwent_r
func _nss_aad_getpwent_r(pwbuf *C.struct_passwd, buf *C.char, buflen C.size_t, errnop *C.int) C.nss_status {
	ctx := ctxWithSyslogLogger(context.Background())
	logger.Debug(ctx, "_nss_aad_getpwent_r called\n")

	p, err := passwd.NextEntry(ctx)
	if err != nil {
		return errToCStatus(ctx, err, errnop)
	}

	if err = p.ToCpasswd(passwd.CPasswd(unsafe.Pointer(pwbuf)), (*passwd.CChar)(buf), passwd.CSizeT(buflen)); err != nil {
		return errToCStatus(ctx, err, errnop)
	}

	return C.NSS_STATUS_SUCCESS
}
