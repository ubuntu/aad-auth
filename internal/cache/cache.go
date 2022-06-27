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

type Cache struct {
	db        *sql.DB
	hasShadow bool

	// offlineLoginValidateFor is the number of days we allow to user to login without online verification.
	// Note that users will be purged from cache when exceeding twice this time.
	offlineLoginValidateFor uint
}

type options struct {
	cacheDir  string
	rootUid   int
	rootGid   int
	shadowGid int // this bypass group lookup

	offlineLoginValidateFor uint
}
type option func(*options) error

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

// WithOfflineLoginValidateFor allows to change the number of days the user can log in without online verification.
// Note that users will be purged from cache when exceeding twice this time.
func WithOfflineLoginValidateFor(days uint) func(o *options) error {
	return func(o *options) error {
		o.offlineLoginValidateFor = days
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
func New(ctx context.Context, opts ...option) (c *Cache, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("can't open/create cache: %v", err)
		}
	}()

	pam.LogDebug(ctx, "Cache initialization")
	var hasShadow bool

	shadowGrp, err := user.LookupGroup("shadow")
	if err != nil {
		return nil, fmt.Errorf("failed to find group id for group shadow: %v", err)
	}
	shadowGid, err := strconv.Atoi(shadowGrp.Gid)
	if err != nil {
		return nil, fmt.Errorf("failed to read shadow group id: %v", err)
	}

	o := options{
		cacheDir:  defaultCachePath,
		rootUid:   0,
		rootGid:   0,
		shadowGid: shadowGid,

		offlineLoginValidateFor: 90,
	}
	// applied options
	for _, opt := range opts {
		if err := opt(&o); err != nil {
			return nil, err
		}
	}

	db, hasShadow, err := initDB(ctx, o.cacheDir, o.rootUid, o.rootGid, o.shadowGid)
	if err != nil {
		return nil, err
	}

	validPeriod := int(time.Duration(2 * c.offlineLoginValidateFor * 24 * uint(time.Hour)))
	if err := cleanUpDB(db, validPeriod); err != nil {
		return nil, err
	}

	pam.LogDebug(ctx, "Attaching shadow db: %v", hasShadow)

	return &Cache{
		db:        db,
		hasShadow: hasShadow,

		offlineLoginValidateFor: o.offlineLoginValidateFor,
	}, nil
}

// Close closes the underlying db.
func (c *Cache) Close() error {
	return c.db.Close()
}

type userRecord struct {
	login            string
	uid              int
	gid              int
	gecos            string
	home             string
	shell            string
	last_online_auth time.Time

	// if shadow is opened
	shadowPasswd string
}

// Update creates and update user nss cache when there has been an online verification.
func (c *Cache) Update(ctx context.Context, username, password string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("can not create/open cache for nss database: %v", err)
		}
	}()

	user, err := c.getUserByName(ctx, username)
	if errors.Is(err, sql.ErrNoRows) {
		// Try creating the user
		id, err := c.generateUidForUser(ctx, username)
		if err != nil {
			return err
		}
		user = userRecord{
			login: username,
			uid:   int(id),
			gid:   int(id),
			home:  filepath.Join("/home", username),
			shell: "/bin/bash", // TODO, check for system default
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
	return c.updateOnlineAuthAndPassword(ctx, user.uid, username, encryptedPassword)
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
		return fmt.Errorf("invalid ownership: %d:%d instead of %d:%d", owner, gOwner, stat.Uid, stat.Gid)
	}

	return nil
}

// encryptPassword returns an encrypted version of password
func encryptPassword(ctx context.Context, username, password string) (string, error) {
	pam.LogDebug(ctx, "encrypt password for user %q", username)

	hash, err := bcrypt.GenerateFromPassword([]byte(username), bcrypt.DefaultCost)
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
