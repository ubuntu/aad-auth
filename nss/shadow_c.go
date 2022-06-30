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
	"errors"
	"fmt"
	"unsafe"

	"github.com/ubuntu/aad-auth/internal/cache"
	"github.com/ubuntu/aad-auth/internal/nss"
	"github.com/ubuntu/aad-auth/internal/shadow"
)

//export _nss_aad_getspnam_r
func _nss_aad_getspnam_r(name *C.char, spwd *C.struct_spwd, buf *C.char, buflen C.size_t, result **C.struct_spwd) C.nss_status {
	n := C.GoString(name)
	sp, err := shadow.NewByName(n)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err) // TODO: log
		return (C.nss_status)(nss.ErrToCStatus(err))
	}
	if err = sp.ToCshadow(
		shadow.CShadow(unsafe.Pointer(spwd)),
		(*shadow.CChar)(buf),
		shadow.CSizeT(buflen),
		(*shadow.CShadow)(unsafe.Pointer(result))); err != nil {
		fmt.Printf("ERROR: %v\n", err) // TODO: log
		return (C.nss_status)(nss.ErrToCStatus(err))
	}

	return C.NSS_STATUS_SUCCESS
}

//export _nss_aad_setspent
func _nss_aad_setspent() {
	// Initialization of the database is done in the read primitive
}

//export _nss_aad_endspent
func _nss_aad_endspent() {
	// Closing the database is done in the read primitive
}

//export _nss_aad_getspent_r
func _nss_aad_getspent_r(spwd *C.struct_spwd, buf *C.char, buflen C.size_t, result **C.struct_spwd) C.nss_status {
	sp, err := shadow.NextEntry()
	if errors.Is(err, cache.ErrNoEnt) {
		return C.ENOENT
	}
	if err != nil {
		fmt.Printf("ERROR: %v\n", err) // TODO: log
		return (C.nss_status)(nss.ErrToCStatus(err))
	}

	if err = sp.ToCshadow(
		shadow.CShadow(unsafe.Pointer(spwd)),
		(*shadow.CChar)(buf),
		shadow.CSizeT(buflen),
		(*shadow.CShadow)(unsafe.Pointer(result))); err != nil {
		fmt.Printf("ERROR: %v\n", err) // TODO: log
		return (C.nss_status)(nss.ErrToCStatus(err))
	}

	return C.NSS_STATUS_SUCCESS
}
