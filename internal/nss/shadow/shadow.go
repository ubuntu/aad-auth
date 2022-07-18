package shadow

import (
	"context"
	"fmt"

	"github.com/ubuntu/aad-auth/internal/cache"
	"github.com/ubuntu/aad-auth/internal/logger"
	"github.com/ubuntu/aad-auth/internal/nss"
)

// Shadow is the nss shadow object.
type Shadow struct {
	name   string /* username */
	passwd string /* user password */
	lstchg int    /* Date of last change */
	min    int    /* Minimum number of days between changes. */
	max    int    /* Maximum number of days between changes. */
	warn   int    /* Number of days to warn user to change the password.  */
	inact  int    /* Number of days the account may be inactive.  */
	expire int    /* Number of days since 1970-01-01 until account expires.  */
}

var testopts = []cache.Option{
	//cache.WithCacheDir("../cache"), cache.WithRootUid(1000), cache.WithRootGid(1000), cache.WithShadowGid(1000),
}

// NewByName returns a passwd entry from a name.
func NewByName(ctx context.Context, name string) (s Shadow, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to get a shadow entry from name %q: %w", name, err)
		}
	}()

	logger.Debug(ctx, "Requesting a shadow entry matching name %q", name)

	c, err := cache.New(ctx, testopts...)
	if err != nil {
		return Shadow{}, nss.ConvertErr(err)
	}
	defer c.Close()

	spw, err := c.GetShadowByName(ctx, name)
	if err != nil {
		return Shadow{}, nss.ConvertErr(err)
	}

	return Shadow{
		name:   spw.Name,
		passwd: "*", // we want to prevent pam_unix using this field to use a cached account without calling pam_aad.
		//passwd: spw.Password,
		lstchg: spw.LastPwdChange,
		min:    spw.MinPwdAge,
		max:    spw.MinPwdAge,
		warn:   spw.MaxPwdAge,
		inact:  spw.PwdInactivity,
		expire: spw.ExpirationDate,
	}, nil
}

var cacheIterateEntries *cache.Cache

// StartEntryIteration open a new cache for iteration.
func StartEntryIteration(ctx context.Context) error {
	c, err := cache.New(ctx, testopts...)
	if err != nil {
		return nss.ConvertErr(err)
	}
	cacheIterateEntries = c

	return nil
}

// EndEntryIteration closes the underlying DB.
func EndEntryIteration(ctx context.Context) error {
	if cacheIterateEntries == nil {
		logger.Warn(ctx, "shadow entry iteration ended without initialization first")
		return nil
	}
	err := cacheIterateEntries.Close()
	cacheIterateEntries = nil
	return err
}

// NextEntry returns next available entry in Shadow. It will returns ENOENT from cache when the iteration is done.
// It automatically opens and close the cache on first/last iteration.
func NextEntry(ctx context.Context) (sp Shadow, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to get a shadow entry: %w", err)
		}
	}()
	logger.Debug(ctx, "get next shadow entry")

	if cacheIterateEntries == nil {
		logger.Warn(ctx, "shadow entry iteration called without initialization first")
		return Shadow{}, nss.ConvertErr(err)
	}

	spw, err := cacheIterateEntries.NextShadowEntry()
	if err != nil {
		return Shadow{}, nss.ConvertErr(err)
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
