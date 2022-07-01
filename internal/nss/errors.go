package nss

import (
	"errors"

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

// ConvertErr converts errors to known types.
func ConvertErr(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, cache.ErrNoEnt) {
		return ErrNotFoundENoEnt
	}

	// TODO: what to do err? Wrapping/logging (would be better to keep it)
	return ErrUnavailableENoEnt
}
