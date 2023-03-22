// Package cache is the package dealing with underlying database and offline/online policy.
package cache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ubuntu/aad-auth/internal/i18n"
	"github.com/ubuntu/aad-auth/internal/logger"
	"github.com/ubuntu/decorate"
	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrNoEnt is returned when there is no entries.
	ErrNoEnt = errors.New("no entries")
	// ErrOfflineCredentialsExpired is returned when the user offline credentials is expired.
	ErrOfflineCredentialsExpired = errors.New("offline credentials expired")
	// ErrOfflineAuthDisabled is returned when offline authentication is disabled by using a negative value in aad.conf.
	ErrOfflineAuthDisabled = errors.New("offline authentication is disabled")
)

const (
	shadowNotAvailableMode = iota
	shadowROMode
	shadowRWMode

	defaultCredentialsExpiration int    = 90
	expirationPurgeMultiplier    uint64 = 2
)

// Cache is the cache object, wrapping our database.
type Cache struct {
	db         *sql.DB
	shadowMode int

	// offlineCredentialsExpiration is the number of days we allow to user to login without online verification.
	// Note that users will be purged from cache when exceeding twice this time.
	offlineCredentialsExpiration int

	cursorPasswd *sql.Rows
	cursorGroup  *sql.Rows
	cursorShadow *sql.Rows

	usedBy           int
	usedByMu         sync.Mutex
	teardownDuration time.Duration
	sig              options // signature used in the cache entry to remove the element from the map.
}

type options struct {
	cacheDir         string
	rootUID          int
	rootGID          int
	shadowGID        int // this bypass group lookup
	forceShadowMode  int // this forces a particular shadow mode
	passwdPermission fs.FileMode
	shadowPermission fs.FileMode
	teardownDuration time.Duration

	offlineCredentialsExpiration int
}

// Option represents the functional option passed to cache.
type Option func(*options) error

// WithCacheDir specifies a personalized cache directory.
func WithCacheDir(p string) func(o *options) error {
	return func(o *options) error {
		o.cacheDir = p
		return nil
	}
}

// WithRootUID allows to change current Root Uid for tests.
func WithRootUID(uid int) func(o *options) error {
	return func(o *options) error {
		o.rootUID = uid
		return nil
	}
}

// WithRootGID allows to change current Root Guid for tests.
func WithRootGID(gid int) func(o *options) error {
	return func(o *options) error {
		o.rootGID = gid
		return nil
	}
}

// WithShadowGID allow change current Shadow Gid for tests.
func WithShadowGID(shadowGID int) func(o *options) error {
	return func(o *options) error {
		o.shadowGID = shadowGID
		return nil
	}
}

// WithShadowMode allow to override current Shadow Gid for tests.
func WithShadowMode(mode int) func(o *options) error {
	return func(o *options) error {
		o.forceShadowMode = mode
		return nil
	}
}

// WithTeardownDuration allows to change current Shadow Gid for tests.
func WithTeardownDuration(d time.Duration) func(o *options) error {
	return func(o *options) error {
		o.teardownDuration = d
		return nil
	}
}

// WithOfflineCredentialsExpiration allows to change the number of days the user can log in without online verification.
// Note that users will be purged from cache when exceeding twice this time.
func WithOfflineCredentialsExpiration(days int) func(o *options) error {
	return func(o *options) error {
		o.offlineCredentialsExpiration = days
		return nil
	}
}

var (
	openedCaches   = make(map[options]*Cache)
	openedCachesMu sync.RWMutex
)

// New returns a new cache handler with the database opened. The cache should be closed and released once unused with .Close().
// For optimization purposes, the cache can stay opened for a while and further New() calls with the same options
// will rematch and reuse the same cache. It will automatically close any released cache on idle.
// There are 2 caches files: one for passwd/group and one for shadow.
// If both does not exists, NewCache will create them with proper permissions only if you are the root user, otherwise
// NewCache will fail.
// If the cache exists, root or members of shadow will open passwd/group and shadow database. Other users will only open
// passwd/group.
// Every open will check for cache ownership and validity permission. If it has been tempered, NewCache will fail.
func New(ctx context.Context, opts ...Option) (c *Cache, err error) {
	defer decorate.OnError(&err, i18n.G("couldn't open/create cache"))

	var shadowMode int

	o := options{
		cacheDir: defaultCachePath,

		rootUID:          0,
		rootGID:          0,
		shadowGID:        -1,
		forceShadowMode:  -1,
		passwdPermission: 0644,
		shadowPermission: 0640,

		teardownDuration: 30 * time.Second,

		offlineCredentialsExpiration: defaultCredentialsExpiration,
	}
	// applied options
	for _, opt := range opts {
		if err := opt(&o); err != nil {
			return nil, err
		}
	}

	initialShadowGID := o.shadowGID

	openedCachesMu.Lock()
	defer openedCachesMu.Unlock()

	// return any used and opened cache
	if c, ok := openedCaches[o]; ok {
		logger.Debug(ctx, "Reusing existing opened cache")
		c.usedByMu.Lock()
		defer c.usedByMu.Unlock()
		c.usedBy++
		return c, nil
	}

	logger.Debug(ctx, "Cache initialization")

	// Only apply shadow lookup here, as in tests, we won’t have a file database available.
	if o.shadowGID < 0 {
		shadowGrp, err := user.LookupGroup("shadow")
		if err != nil {
			return nil, fmt.Errorf(i18n.G("failed to find group id for group shadow: %w"), err)
		}
		o.shadowGID, err = strconv.Atoi(shadowGrp.Gid)
		if err != nil {
			return nil, fmt.Errorf(i18n.G("failed to read shadow group id: %w"), err)
		}
	}

	db, shadowMode, err := initDB(ctx, o.cacheDir, o.rootUID, o.rootGID, o.shadowGID, o.forceShadowMode, o.passwdPermission, o.shadowPermission)
	if err != nil {
		return nil, err
	}

	logger.Debug(ctx, "Shadow db mode: %v", shadowMode)

	if shadowMode == shadowRWMode && o.offlineCredentialsExpiration != 0 {
		d := o.offlineCredentialsExpiration
		if o.offlineCredentialsExpiration < 0 {
			d = defaultCredentialsExpiration
		}
		days := uint64(d) * expirationPurgeMultiplier

		maxCacheEntryDuration := time.Duration(days * 24 * uint64(time.Hour))
		if err := cleanUpDB(ctx, db, maxCacheEntryDuration); err != nil {
			return nil, err
		}
	} else if o.offlineCredentialsExpiration == 0 {
		logger.Debug(ctx, "Cache won't be cleaned up as credentials expiration is set to 0")
	}

	// reset shadowGid to initial value as the detection may have changed it after initialization, to retest
	o.shadowGID = initialShadowGID

	c = &Cache{
		db:         db,
		shadowMode: shadowMode,

		offlineCredentialsExpiration: o.offlineCredentialsExpiration,

		usedBy:           1,
		teardownDuration: o.teardownDuration,
		sig:              o,
	}
	openedCaches[o] = c

	return c, nil
}

// Close closes the underlying db.
// After a while, if no other connection to this db is active, this cache will be closed.
func (c *Cache) Close(ctx context.Context) error {
	logger.Debug(ctx, "Close database request")

	c.usedByMu.Lock()
	defer c.usedByMu.Unlock()
	c.usedBy--
	if c.usedBy != 0 {
		return nil
	}

	// Start closing DB timer.
	go func() {
		<-time.After(c.teardownDuration)

		// Take master mutex to avoid getting a cached object we may remove from the cache map
		openedCachesMu.Lock()
		defer openedCachesMu.Unlock()

		// Teardown underlying connection if there is still no usage
		c.usedByMu.Lock()
		defer c.usedByMu.Unlock()
		if c.usedBy != 0 {
			if c.usedBy > 0 {
				logger.Debug(ctx, "Don't teardown cache as still in use by %d", c.usedBy)
			}
			return
		}

		logger.Debug(ctx, "No use of cache, closing underlying DB.")

		if err := c.ClosePasswdIterator(ctx); err != nil {
			logger.Warn(ctx, "%v", err)
		}
		if err := c.CloseGroupIterator(ctx); err != nil {
			logger.Warn(ctx, "%v", err)
		}
		if err := c.CloseShadowIterator(ctx); err != nil {
			logger.Warn(ctx, "%v", err)
		}

		if err := c.db.Close(); err != nil {
			logger.Warn(ctx, "%v", err)
		}

		delete(openedCaches, c.sig)
		c.usedBy = -1 // prevent other consumer in close teardown to clear it up
	}()

	return nil
}

// CanAuthenticate tries to authenticates user from cache and check it hasn't expired.
// It returns an error if it can’t authenticate.
func (c *Cache) CanAuthenticate(ctx context.Context, username, password string) (err error) {
	defer decorate.OnError(&err, i18n.G("authenticating user %q from cache failed"), username)

	logger.Info(ctx, "try to authenticate %q from cache", username)

	if c.shadowMode < shadowROMode {
		return errors.New("shadow database is not available for reading")
	}

	if c.offlineCredentialsExpiration < 0 {
		return ErrOfflineAuthDisabled
	}

	user, err := c.GetUserByName(ctx, username)
	if err != nil {
		return err
	}

	// ensure that we checked credential online recently.
	logger.Debug(ctx, "Last online login was: %s. Current time: %s.", user.LastOnlineAuth, time.Now())
	if c.offlineCredentialsExpiration > 0 {
		logger.Debug(ctx, "Online revalidation needed every %d days", c.offlineCredentialsExpiration)
		if time.Now().After(user.LastOnlineAuth.Add(time.Duration(uint64(c.offlineCredentialsExpiration) * 24 * uint64(time.Hour)))) {
			return ErrOfflineCredentialsExpired
		}
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.ShadowPasswd), []byte(password)); err != nil {
		return fmt.Errorf("password does not match: %w", err)
	}

	return nil
}

// Update creates and update user nss cache when there has been an online verification.
func (c *Cache) Update(ctx context.Context, username, password, homeDirPattern, shell string) (err error) {
	defer decorate.OnError(&err, i18n.G("couldn't create/open cache for nss database"))

	user, err := c.GetUserByName(ctx, username)
	if errors.Is(err, ErrNoEnt) {
		// Try creating the user
		id, err := c.generateUIDForUser(ctx, username)
		if err != nil {
			return err
		}

		home, err := parseHomeDir(ctx, homeDirPattern, username, fmt.Sprintf("%d", id))
		if err != nil {
			return err
		}

		user = UserRecord{
			Name:  username,
			UID:   int64(id),
			GID:   int64(id),
			Home:  home,
			Shell: shell,
		}

		if err := c.insertUser(ctx, user); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	encryptedPassword, err := encryptPassword(ctx, username, password)
	if err != nil {
		return err
	}
	return c.updateOnlineAuthAndPassword(ctx, user.UID, username, encryptedPassword)
}

// checkFilePermission ensure that the file has correct ownership and permissions.
func checkFilePermission(ctx context.Context, p string, owner, gOwner int, permission fs.FileMode) (err error) {
	defer decorate.OnError(&err, i18n.G("failed checking file permission for %s"), p)

	logger.Debug(ctx, "check file permissions on %v", p)

	info, err := os.Stat(p)
	if err != nil {
		return err
	}
	if info.Mode() != permission {
		return fmt.Errorf("invalid file permission: %s instead of %s", info.Mode(), permission)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("can't get ownership of file")
	}

	if owner != int(stat.Uid) || gOwner != int(stat.Gid) {
		return fmt.Errorf("invalid ownership: %d:%d instead of %d:%d", stat.Uid, stat.Gid, owner, gOwner)
	}

	return nil
}

// encryptPassword returns an encrypted version of password.
func encryptPassword(ctx context.Context, username, password string) (string, error) {
	logger.Debug(ctx, "encrypt password for user %q", username)

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt password: %w", err)
	}
	return string(hash), nil
}

// generateUIDForUser returns an unique uid for the user to create.
func (c *Cache) generateUIDForUser(ctx context.Context, username string) (uid uint32, err error) {
	defer decorate.OnError(&err, i18n.G("failed to generate uid for user %q"), username)

	logger.Debug(ctx, "generate user id for user %q", username)

	// compute uid for user
	var offset uint32 = 100000
	uid = 1
	for _, c := range username {
		uid = (uid * uint32(c)) % math.MaxUint32
	}
	uid = uid%(math.MaxUint32-offset) + offset

	// check collision or increment
	for {
		if exists, err := uidOrGidExists(c.db, uid, username); err != nil {
			return 0, err
		} else if exists {
			uid++
			continue
		}

		break
	}

	logger.Info(ctx, "user id for %q is %d", username, uid)

	return uid, nil
}

// ShadowReadable returns true if shadow database is readable.
func (c *Cache) ShadowReadable() bool {
	return c.shadowMode > shadowNotAvailableMode
}

// parseHomeDir returns a string containing the homedir path generated by parsing homeDirPattern.
// In case there are any invalid modifiers, returns an error.
func parseHomeDir(ctx context.Context, homeDirPattern, username, uid string) (home string, err error) {
	defer decorate.OnError(&err, i18n.G("couldn't parse home directory"))

	logger.Debug(ctx, "Getting home directory for %s", username)

	var afterModifier bool
	for _, c := range homeDirPattern {
		s := string(c)

		// treat % and %% cases.
		if c == '%' && !afterModifier {
			afterModifier = true
			continue
		} else if c == '%' && afterModifier {
			afterModifier = false // second % is now considered as a "normal" character to append.
		}

		// treat special modifiers.
		if afterModifier {
			s, err = parseHomeDirPattern(s, username, uid)
			if err != nil {
				return "", err
			}
		}

		// append converted characters or normal ones to the path, as if.
		afterModifier = false
		home += s
	}
	return home, nil
}

// parseHomeDirPattern returns the string that matches the given pattern.
// If the pattern is not recognized, an error is returned.
func parseHomeDirPattern(pattern, username, uid string) (string, error) {
	switch pattern {
	case "f":
		return username, nil
	case "U":
		return uid, nil
	case "l":
		return string(username[0]), nil
	case "u", "d":
		user, domain, _ := strings.Cut(username, "@")
		if pattern == "u" {
			return user, nil
		}
		return domain, nil
	}
	return "", fmt.Errorf("%%%s is not a valid pattern", pattern)
}
