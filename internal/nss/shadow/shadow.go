package shadow

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/ubuntu/aad-auth/internal/cache"
	"github.com/ubuntu/aad-auth/internal/i18n"
	"github.com/ubuntu/aad-auth/internal/logger"
	"github.com/ubuntu/aad-auth/internal/nss"
	"github.com/ubuntu/decorate"
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

// NewByName returns a passwd entry from a name.
func NewByName(ctx context.Context, name string, cacheOpts ...cache.Option) (s Shadow, err error) {
	defer decorate.OnError(&err, i18n.G("failed to get a shadow entry from name %q"), name)

	logger.Debug(ctx, "Requesting a shadow entry matching name %q", name)

	c, err := cache.New(ctx, cacheOpts...)
	if err != nil {
		return Shadow{}, nss.ConvertErr(err)
	}
	defer c.Close(ctx)

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
		max:    spw.MaxPwdAge,
		warn:   spw.PwdWarnPeriod,
		inact:  spw.PwdInactivity,
		expire: spw.ExpirationDate,
	}, nil
}

// String creates a string with Shadow values.
func (s Shadow) String() string {
	v := []string{
		s.name,
		s.passwd,
		strconv.Itoa(s.lstchg),
		strconv.Itoa(s.min),
		strconv.Itoa(s.max),
		strconv.Itoa(s.warn),
		strconv.Itoa(s.inact),
		strconv.Itoa(s.expire),
		strconv.FormatUint(^uint64(0), 10),
	}
	return strings.Join(v, ":")
}

var shadowIterationCache *cache.Cache

// StartEntryIteration open a new cache for iteration.
// This needs to be called prior to calling NextEntry and be closed with EndEntryIteration.
func StartEntryIteration(ctx context.Context, cacheOpts ...cache.Option) error {
	if shadowIterationCache != nil {
		return nss.ConvertErr(errors.New("shadow entry iteration already in progress. End it before starting a new one"))
	}

	c, err := cache.New(ctx, cacheOpts...)
	if err != nil {
		return nss.ConvertErr(err)
	}
	shadowIterationCache = c
	return nil
}

// EndEntryIteration closes the underlying DB iterator.
func EndEntryIteration(ctx context.Context) error {
	if shadowIterationCache == nil {
		logger.Warn(ctx, "shadow entry iteration ended without initialization first")
		return nil
	}
	c := shadowIterationCache
	defer c.Close(ctx)
	shadowIterationCache = nil
	return nss.ConvertErr(c.CloseShadowIterator(ctx))
}

// NextEntry returns next available entry in Shadow. It will returns ENOENT from cache when the iteration is done.
// It automatically opens and close the cache on first/last iteration.
func NextEntry(ctx context.Context) (sp Shadow, err error) {
	defer decorate.OnError(&err, i18n.G("failed to get a shadow entry"))

	logger.Debug(ctx, "get next shadow entry")

	if shadowIterationCache == nil {
		return Shadow{}, nss.ConvertErr(errors.New("shadow entry iteration called without initialization first"))
	}

	spw, err := shadowIterationCache.NextShadowEntry(ctx)
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
