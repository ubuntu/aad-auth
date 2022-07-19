package nss

import (
	"errors"
	"fmt"

	"github.com/ubuntu/aad-auth/internal/cache"
)

var (
	// ErrTryAgainEAgain matches NSS status TRYAGAIN. One of the functions used ran temporarily out of resources or a service is currently not available.
	ErrTryAgainEAgain = errors.New("try again EAGAIN")
	// ErrTryAgainERange matched NSS status ERANGE. The provided buffer is not large enough. The function should be called again with a larger buffer.
	ErrTryAgainERange = errors.New("try again ERANGE")
	// ErrUnavailableENoEnt matches NSS status UNAVAIL. A necessary input file cannot be found.
	ErrUnavailableENoEnt = errors.New("unavailable ENOENT")
	// ErrNotFoundENoEnt matches NSS status NOTFOUND. The requested entry is not available.
	ErrNotFoundENoEnt = errors.New("not found ENOENT")
	// ErrNotFoundSuccess matches NSS status NOTFOUND. There are no entries. Use this to avoid returning errors for inactive services which may be enabled at a later time. This is not the same as the service being temporarily unavailable.
	ErrNotFoundSuccess = errors.New("not found SUCCESS")
)

type nssError struct {
	origErr error
	apiErr  error
}

func (err nssError) Unwrap() error {
	return err.apiErr
}

func (err nssError) Error() string {
	return fmt.Sprintf("%v (returning %v)", err.origErr, err.apiErr)
}

// ConvertErr converts errors to known types, wrapping the original ones.
func ConvertErr(origErr error) error {
	if origErr == nil {
		return nil
	}

	apiErr := ErrUnavailableENoEnt

	// If we have an API error already, do not override it.
	if errors.Is(origErr, ErrTryAgainEAgain) ||
		errors.Is(origErr, ErrTryAgainERange) ||
		errors.Is(origErr, ErrUnavailableENoEnt) ||
		errors.Is(origErr, ErrNotFoundENoEnt) ||
		errors.Is(origErr, ErrNotFoundSuccess) {
		apiErr = origErr
	} else if errors.Is(origErr, cache.ErrNoEnt) {
		// Special error from cache to convert.
		apiErr = ErrNotFoundENoEnt
	}

	return nssError{
		origErr: origErr,
		apiErr:  apiErr,
	}
}
