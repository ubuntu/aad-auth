package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/ubuntu/aad-auth/internal/cache"
	"github.com/ubuntu/aad-auth/internal/logger"
	"github.com/ubuntu/aad-auth/internal/nss"
	"github.com/ubuntu/aad-auth/internal/nss/group"
	"github.com/ubuntu/aad-auth/internal/nss/passwd"
	"github.com/ubuntu/aad-auth/internal/nss/shadow"
)

var supportedDbs = []string{"group", "passwd", "shadow"}

// Getent processes the args and queries the database for the requested entries.
func Getent(ctx context.Context, dbName, key string, cacheOpts ...cache.Option) (string, error) {
	if !dbIsSupported(dbName) {
		return "", fmt.Errorf("database %q is not supported", dbName)
	}

	logger.Debug(ctx, "Getting entry %q from %s ", key, dbName)

	var entries []fmt.Stringer
	var err error
	if key != "" {
		var e fmt.Stringer
		e, err = getEntryByKey(ctx, dbName, key, cacheOpts...)
		entries = []fmt.Stringer{e}
		if err != nil {
			entries = nil
		}
	} else {
		entries, err = getAllEntries(ctx, dbName, cacheOpts...)
	}

	return fmtGetentOutput(ctx, entries, err), nil
}

func getEntryByKey(ctx context.Context, dbName, key string, cacheOpts ...cache.Option) (entry fmt.Stringer, err error) {
	u, err := strconv.ParseUint(key, 10, 64)
	if err != nil {
		return getEntryByName(ctx, dbName, key, cacheOpts...)
	}
	return getEntryByID(ctx, dbName, uint(u), cacheOpts...)
}

func getEntryByName(ctx context.Context, dbName, name string, cacheOpts ...cache.Option) (entry fmt.Stringer, err error) {
	logger.Debug(ctx, "Getting entry with name %q from %s", name, dbName)

	var e fmt.Stringer

	switch dbName {
	case "passwd":
		e, err = passwd.NewByName(ctx, name, cacheOpts...)
	case "group":
		e, err = group.NewByName(ctx, name, cacheOpts...)
	case "shadow":
		e, err = shadow.NewByName(ctx, name, cacheOpts...)
	}

	if err != nil {
		return nil, err
	}

	return e, nil
}

func getEntryByID(ctx context.Context, dbName string, id uint, cacheOpts ...cache.Option) (entry fmt.Stringer, err error) {
	logger.Debug(ctx, "Getting entry with id %q from %s", id, dbName)

	var e fmt.Stringer

	switch dbName {
	case "passwd":
		e, err = passwd.NewByUID(ctx, id, cacheOpts...)
	case "group":
		e, err = group.NewByGID(ctx, id, cacheOpts...)
	case "shadow":
		return nil, nss.ErrNotFoundENoEnt
	}

	if err != nil {
		return nil, err
	}

	return e, nil
}

func getAllEntries(ctx context.Context, dbName string, cacheOpts ...cache.Option) (entries []fmt.Stringer, err error) {
	logger.Debug(ctx, "Getting all entries from %s", dbName)

	start, end := initIterationForDB(dbName)
	if err = start(ctx, cacheOpts...); err != nil {
		return nil, err
	}
	defer end(ctx) //nolint:errcheck // We know that this is a call for EndIteration and we don't need to check the return.

	for {
		var entry fmt.Stringer
		entry, err = nextEntryForDB(ctx, dbName)
		if err != nil {
			// The iteration ends with ErrNoEnt, even when it's successful.
			break
		}
		entries = append(entries, entry)
	}

	if !errors.Is(err, nss.ErrNotFoundENoEnt) {
		return nil, err
	}

	// If error is ENoEnt and we got no entries, this means the cache is empty and the nss error returned should be ErrNotFoundSuccess.
	if len(entries) == 0 {
		return nil, nss.ErrNotFoundSuccess
	}

	return entries, nil
}

func initIterationForDB(dbName string) (func(ctx context.Context, opts ...cache.Option) error, func(ctx context.Context) error) {
	switch dbName {
	case "passwd":
		return passwd.StartEntryIteration, passwd.EndEntryIteration
	case "group":
		return group.StartEntryIteration, group.EndEntryIteration
	case "shadow":
		return shadow.StartEntryIteration, shadow.EndEntryIteration
	}
	return nil, nil
}

func nextEntryForDB(ctx context.Context, dbName string) (fmt.Stringer, error) {
	var entry fmt.Stringer
	var err error
	switch dbName {
	case "passwd":
		entry, err = passwd.NextEntry(ctx)
	case "group":
		entry, err = group.NextEntry(ctx)
	case "shadow":
		entry, err = shadow.NextEntry(ctx)
	}
	return entry, err
}

func fmtGetentOutput(ctx context.Context, entries []fmt.Stringer, err error) string {
	var out string

	status, errno := errToCStatus(ctx, err)
	out = fmt.Sprintf("%d:%d", status, errno)

	for _, entry := range entries {
		out = fmt.Sprintf("%s\n%s", out, entry)
	}

	return out
}

func dbIsSupported(db string) bool {
	for _, d := range supportedDbs {
		if d == db {
			return true
		}
	}
	return false
}
