package nss

/*
#include <nss.h>

typedef enum nss_status nss_status;
*/
import "C"
import (
	"errors"

	"github.com/ubuntu/aad-auth/internal/cache"
)

var (
	// ErrTryAgain match NSS status TRYAGAIN
	ErrTryAgain = errors.New("try again")
	// ErrTryAgain match NSS status UNAVAIL
	ErrUnavailable = errors.New("unavailable")
	// ErrTryAgain match NSS status NOTFOUND
	ErrNotFound = errors.New("not found")
)

// ErrNoEntriesToNotFound converts cache error for no entries to a not found one.
func ErrNoEntriesToNotFound(err error) error {
	if errors.Is(err, cache.ErrNoEnt) {
		return ErrNotFound
	}
	return err
}

// ErrToCStatus converts our Go errors to corresponding nss status returned code.
// If err is nil, it returns a success.
func ErrToCStatus(err error) C.nss_status {
	if errors.Is(err, ErrTryAgain) {
		return C.NSS_STATUS_TRYAGAIN
	} else if errors.Is(err, ErrUnavailable) {
		return C.NSS_STATUS_UNAVAIL
	} else if errors.Is(err, ErrNotFound) {
		return C.NSS_STATUS_NOTFOUND
	} else if err != nil { // By default: system error
		return C.NSS_STATUS_UNAVAIL
	}

	return C.NSS_STATUS_SUCCESS
}
