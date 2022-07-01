package shadow

import (
	"context"
	"errors"
	"fmt"

	"github.com/ubuntu/aad-auth/internal/cache"
	"github.com/ubuntu/aad-auth/internal/nss"
	"github.com/ubuntu/aad-auth/internal/pam"
)

type Shadow struct {
	name   string /* username */
	passwd string /* user password */
	lstchg uint   /* Date of last change */
	min    uint   /* Minimum number of days between changes. */
	max    uint   /* Maximum number of days between changes. */
	warn   uint   /* Number of days to warn user to change the password.  */
	inact  uint   /* Number of days the account may be inactive.  */
	expire uint   /* Number of days since 1970-01-01 until account expires.  */
}

var testopts = []cache.Option{
	//cache.WithCacheDir("../cache"), cache.WithRootUid(1000), cache.WithRootGid(1000), cache.WithShadowGid(1000),
}

// NewByName returns a passwd entry from a name.
func NewByName(name string) (s Shadow, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to get a shadow entry from name %q: %v", name, err)
		}
	}()

	ctx := context.Background()
	pam.LogDebug(context.Background(), "Requesting a shadow entry matching name %q", name)

	c, err := cache.New(ctx, testopts...)
	if err != nil {
		return Shadow{}, nss.ErrUnavailable
	}
	defer c.Close()

	spw, err := c.GetShadowByName(ctx, name)
	if err != nil {
		return Shadow{}, nss.ErrNoEntriesToNotFound(err)
	}

	return Shadow{
		name:   spw.Name,
		passwd: spw.Password,
		lstchg: spw.LastPwdChange,
		min:    spw.MinPwdAge,
		max:    spw.MinPwdAge,
		warn:   spw.MaxPwdAge,
		inact:  spw.PwdInactivity,
		expire: spw.ExpirationDate,
	}, nil
}

var cacheIterateEntries *cache.Cache

// NextEntry returns next available entry in Shadow. It will returns ENOENT from cache when the iteration is done.
// It automatically opens and close the cache on first/last iteration.
func NextEntry() (sp Shadow, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to get a shadow entry: %v", err)
		}
	}()
	pam.LogDebug(context.Background(), "get next shadow entry")

	if cacheIterateEntries == nil {
		cacheIterateEntries, err = cache.New(context.Background(), testopts...)
		if err != nil {
			return Shadow{}, err
		}
	}

	spw, err := cacheIterateEntries.NextShadowEntry()
	if errors.Is(err, cache.ErrNoEnt) {
		_ = cacheIterateEntries.Close()
		cacheIterateEntries = nil
		return Shadow{}, err
	} else if err != nil {
		return Shadow{}, err
	}

	return Shadow{
		name:   spw.Name,
		passwd: spw.Password,
		lstchg: spw.LastPwdChange,
		min:    spw.MinPwdAge,
		max:    spw.MaxPwdAge,
		warn:   spw.PwdWarnPeriod,
		inact:  spw.PwdInactivity,
		expire: spw.ExpirationDate,
	}, nil
}
