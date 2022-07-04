package main

/*
#cgo LDFLAGS: -fPIC
#include <nss.h>
#include <shadow.h>
#include <errno.h>

typedef enum nss_status nss_status;
*/
import "C"
import (
	"fmt"
	"unsafe"

	"github.com/ubuntu/aad-auth/internal/nss/shadow"
)

//export _nss_aad_getspnam_r
func _nss_aad_getspnam_r(name *C.char, spwd *C.struct_spwd, buf *C.char, buflen C.size_t, errnop *C.int) C.nss_status {
	n := C.GoString(name)
	sp, err := shadow.NewByName(n)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err) // TODO: log
		return errToCStatus(err, errnop)
	}
	if err = sp.ToCshadow(shadow.CShadow(unsafe.Pointer(spwd)), (*shadow.CChar)(buf), shadow.CSizeT(buflen)); err != nil {
		fmt.Printf("ERROR: %v\n", err) // TODO: log
		return errToCStatus(err, errnop)
	}

	return C.NSS_STATUS_SUCCESS
}

//export _nss_aad_setspent
func _nss_aad_setspent() C.nss_status {
	// Initialization of the database is done in the read primitive
	return C.NSS_STATUS_SUCCESS
}

//export _nss_aad_endspent
func _nss_aad_endspent() C.nss_status {
	// Closing the database is done in the read primitive
	return C.NSS_STATUS_SUCCESS
}

//export _nss_aad_getspent_r
func _nss_aad_getspent_r(spwd *C.struct_spwd, buf *C.char, buflen C.size_t, errnop *C.int) C.nss_status {
	sp, err := shadow.NextEntry()
	if err != nil {
		fmt.Printf("ERROR: %v\n", err) // TODO: log
		return errToCStatus(err, errnop)
	}

	if err = sp.ToCshadow(shadow.CShadow(unsafe.Pointer(spwd)), (*shadow.CChar)(buf), shadow.CSizeT(buflen)); err != nil {
		fmt.Printf("ERROR: %v\n", err) // TODO: log
		return errToCStatus(err, errnop)
	}

	return C.NSS_STATUS_SUCCESS
}
