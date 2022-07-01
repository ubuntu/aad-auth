package main

/*
#include <errno.h>
#include <nss.h>

typedef enum nss_status nss_status;
*/
import "C"
import (
	"errors"

	"github.com/ubuntu/aad-auth/internal/nss"
)

// errToCStatus converts our Go errors to corresponding nss status returned code and errno.
// If err is nil, it returns a success.
func errToCStatus(err error, errnop *C.int) C.nss_status {
	var nssStatus C.nss_status = C.NSS_STATUS_SUCCESS
	var errno int

	switch {
	case errors.Is(err, nss.ErrTryAgainEAgain):
		nssStatus = C.NSS_STATUS_TRYAGAIN
		errno = C.EAGAIN
	case errors.Is(err, nss.ErrTryAgainERange):
		nssStatus = C.NSS_STATUS_TRYAGAIN
		errno = C.ERANGE
	case errors.Is(err, nss.ErrUnavailableENoEnt):
		nssStatus = C.NSS_STATUS_UNAVAIL
		errno = C.ENOENT
	case errors.Is(err, nss.ErrNotFoundENoEnt):
		nssStatus = C.NSS_STATUS_NOTFOUND
		errno = C.ENOENT
	case errors.Is(err, nss.ErrNotFoundSuccess):
		nssStatus = C.NSS_STATUS_SUCCESS
		errno = C.ENOENT
	case err != nil: // Unexpected returned error
		nssStatus = C.NSS_STATUS_SUCCESS
		errno = C.EINVAL
	}

	*errnop = C.int(errno)

	return nssStatus
}
