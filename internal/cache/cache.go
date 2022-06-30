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
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"

	"github.com/ubuntu/aad-auth/internal/pam"
)

var (
	// ErrNoEnt is returned when there is no entries
	ErrNoEnt = errors.New("No entries")
)

type Cache struct {
	db        *sql.DB
	hasShadow bool

	// revalidationPeriod is the number of days we allow to user to login without online verification.
	// Note that users will be purged from cache when exceeding twice this time.
	revalidationPeriod int

	cursorPasswd *sql.Rows
	cursorGroup  *sql.Rows
	cursorShadow *sql.Rows
}

type options struct {
	cacheDir  string
	rootUid   int
	rootGid   int
	shadowGid int // this bypass group lookup

	revalidationPeriod int
}
type Option func(*options) error

// WithCacheDir specifies a personalized cache directory.
func WithCacheDir(p string) func(o *options) error {
	return func(o *options) error {
		o.cacheDir = p
		return nil
	}
}

//////////////////// to move for tests

// WithRootUid allows to change current Root Uid for tests
func WithRootUid(uid int) func(o *options) error {
	return func(o *options) error {
		o.rootUid = uid
		return nil
	}
}

// WithRootGid allows to change current Root Guid for tests
func WithRootGid(gid int) func(o *options) error {
	return func(o *options) error {
		o.rootGid = gid
		return nil
	}
}

// WithShadowGid allow change current Shadow Gid for tests
func WithShadowGid(shadowGid int) func(o *options) error {
	return func(o *options) error {
		o.shadowGid = shadowGid
		return nil
	}
}

// WithRevalidationPeriod allows to change the number of days the user can log in without online verification.
// Note that users will be purged from cache when exceeding twice this time.
func WithRevalidationPeriod(days int) func(o *options) error {
	return func(o *options) error {
		if days < 0 {
			return nil
		}
		o.revalidationPeriod = days
		return nil
	}
}

// NewCache returns a new cache handler with the database opens. The cache should be closed once unused with .Close()
// There are 2 caches files: one for passwd/group and one for shadow.
// If both does not exists, NewCache will create them with proper permissions only if you are the root user, otherwise
// NewCache will fail.
// If the cache exists, root or members of shadow will open passwd/group and shadow database. Other users will only open
// passwd/group.
// Every open will check for cache ownership and validity permission. If it has been tempered, NewCache will fail.
func New(ctx context.Context, opts ...Option) (c *Cache, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("can't open/create cache: %v", err)
		}
	}()

	pam.LogDebug(ctx, "Cache initialization")
	var hasShadow bool

	o := options{
		cacheDir:  defaultCachePath,
		rootUid:   0,
		rootGid:   0,
		shadowGid: -1,

		revalidationPeriod: 90,
	}
	// applied options
	for _, opt := range opts {
		if err := opt(&o); err != nil {
			return nil, err
		}
	}

	// Only apply shadow lookup here, as in tests, we won’t have a file database available.
	if o.shadowGid < 0 {
		shadowGrp, err := user.LookupGroup("shadow")
		if err != nil {
			return nil, fmt.Errorf("failed to find group id for group shadow: %v", err)
		}
		o.shadowGid, err = strconv.Atoi(shadowGrp.Gid)
		if err != nil {
			return nil, fmt.Errorf("failed to read shadow group id: %v", err)
		}
	}

	db, hasShadow, err := initDB(ctx, o.cacheDir, o.rootUid, o.rootGid, o.shadowGid)
	if err != nil {
		return nil, err
	}

	pam.LogDebug(ctx, "Attaching shadow db: %v", hasShadow)

	if hasShadow {
		revalidationPeriodDuration := time.Duration(2 * uint(o.revalidationPeriod) * 24 * uint(time.Hour))
		if err := cleanUpDB(ctx, db, revalidationPeriodDuration); err != nil {
			return nil, err
		}
	}

	return &Cache{
		db:        db,
		hasShadow: hasShadow,

		revalidationPeriod: o.revalidationPeriod,
	}, nil
}

// Close closes the underlying db.
func (c *Cache) Close() error {
	if c.cursorPasswd != nil {
		_ = c.cursorPasswd.Close()
	}
	if c.cursorGroup != nil {
		_ = c.cursorGroup.Close()
	}
	if c.cursorShadow != nil {
		_ = c.cursorShadow.Close()
	}
	return c.db.Close()
}

// UserRecord returns a user record from the cache
type UserRecord struct {
	Name           string
	Passwd         string
	UID            int
	GID            int
	Gecos          string
	Home           string
	Shell          string
	LastOnlineAuth time.Time

	// if shadow is opened
	ShadowPasswd string
}

// GroupRecord  returns a group record from the cache
type GroupRecord struct {
	Name     string
	GID      int
	Password string
	Members  []string
}

// ShadowRecord returns a shadow record from the cache
type ShadowRecord struct {
	Name           string
	Password       string
	LastPwdChange  int
	MaxPwdAge      int
	PwdWarnPeriod  int
	PwdInactivity  int
	MinPwdAge      int
	ExpirationDate int
}

// CanAuthenticate tries to authenticates user from cache and check it hasn't expired.
// It returns an error if it can’t authenticate
func (c *Cache) CanAuthenticate(ctx context.Context, username, password string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("authenticating user %q from cache failed: %v", username, err)
		}
	}()

	pam.LogInfo(ctx, "try to authenticate %q from cache", username)

	if !c.hasShadow {
		return errors.New("shadow database is not available")
	}

	user, err := c.GetUserByName(ctx, username)
	if err != nil {
		return err
	}

	// ensure that we checked credential online recently.
	pam.LogDebug(ctx, "Last online login was: %s. Current time: %s. Revalidation needed every %d days", user.LastOnlineAuth, time.Now(), c.revalidationPeriod)
	if time.Now().After(user.LastOnlineAuth.Add(time.Duration(uint(c.revalidationPeriod) * 24 * uint(time.Hour)))) {
		return errors.New("cache expired")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.ShadowPasswd), []byte(password)); err != nil {
		return errors.New("password does not match: %v")
	}

	return nil
}

// Update creates and update user nss cache when there has been an online verification.
func (c *Cache) Update(ctx context.Context, username, password string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("can not create/open cache for nss database: %v", err)
		}
	}()

	user, err := c.GetUserByName(ctx, username)
	if errors.Is(err, ErrNoEnt) {
		// Try creating the user
		id, err := c.generateUidForUser(ctx, username)
		if err != nil {
			return err
		}
		user = UserRecord{
			Name:  username,
			UID:   int(id),
			GID:   int(id),
			Home:  filepath.Join("/home", username),
			Shell: "/bin/bash", // TODO, check for system default
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
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed checking file permission for %v: %v", p, err)
		}
	}()
	pam.LogDebug(ctx, "check file permissions on %v", p)

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

// encryptPassword returns an encrypted version of password
func encryptPassword(ctx context.Context, username, password string) (string, error) {
	pam.LogDebug(ctx, "encrypt password for user %q", username)

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt password: %v", err)
	}
	return string(hash), nil
}

// generateUidForUser returns an unique uid for the user to create.
func (c *Cache) generateUidForUser(ctx context.Context, username string) (uid uint32, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to generate uid for user %q: %v", username, err)
		}
	}()

	pam.LogDebug(ctx, "generate user id for user %q", username)

	// compute uid for user
	var offset uint32 = 100000
	uid = 1
	for _, c := range []rune(username) {
		uid = (uid * uint32(c)) % math.MaxUint32
	}
	uid = uid%(math.MaxUint32-offset) + offset

	// check collision or increment
	for {
		if exists, err := uidOrGidExists(c.db, uid, username); err != nil {
			return 0, err
		} else if exists {
			uid += 1
			continue
		}

		break
	}

	pam.LogInfo(ctx, "user id for %q is %d", username, uid)

	return uid, nil
}
