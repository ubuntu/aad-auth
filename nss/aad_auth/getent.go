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

// GetEnt processes the args and queries the database for the requested entries.
func GetEnt(ctx context.Context, dbName, key string, cacheOpts ...cache.Option) (entries []string, err error) {
	logger.Debug(ctx, "Getting entry %s from %s ", key, dbName)

	if key != "" {
		e, err := getEntryByKey(ctx, dbName, key, cacheOpts...)
		if err != nil {
			return nil, err
		}
		return []string{e}, nil
	}
	return getAllEntries(ctx, dbName, cacheOpts...)
}

func getEntryByKey(ctx context.Context, dbName, key string, cacheOpts ...cache.Option) (entry string, err error) {
	u, err := strconv.ParseUint(key, 10, 64)
	if err != nil {
		return getEntryByName(ctx, dbName, key, cacheOpts...)
	}
	return getEntryByID(ctx, dbName, uint(u), cacheOpts...)
}

func getEntryByName(ctx context.Context, dbName, name string, cacheOpts ...cache.Option) (entry string, err error) {
	logger.Debug(ctx, "Getting entry by name")

	var e stringer

	switch dbName {
	case "passwd":
		e, err = passwd.NewByName(ctx, name, cacheOpts...)
	case "group":
		e, err = group.NewByName(ctx, name, cacheOpts...)
	case "shadow":
		e, err = shadow.NewByName(ctx, name, cacheOpts...)
	}

	if err != nil {
		return "", err
	}

	return e.String(), nil
}

func getEntryByID(ctx context.Context, dbName string, id uint, cacheOpts ...cache.Option) (entry string, err error) {
	logger.Debug(ctx, "Getting entry by id")

	var e stringer

	switch dbName {
	case "passwd":
		e, err = passwd.NewByUID(ctx, id, cacheOpts...)
	case "group":
		e, err = group.NewByGID(ctx, id, cacheOpts...)
	case "shadow":
		return "", fmt.Errorf("Shadow db does not support getting entries by ID")
	}

	if err != nil {
		return "", err
	}

	return e.String(), nil
}

func getAllEntries(ctx context.Context, dbName string, cacheOpts ...cache.Option) (entries []string, err error) {
	logger.Debug(ctx, "Getting all entries")

	defer func() {
		if !errors.Is(err, nss.ErrNotFoundENoEnt) {
			entries = nil
			return
		}
		// if we have entries, do not return ErrNoEnt, the C library will do it on the last iteration.
		if len(entries) != 0 {
			err = nil
		}
	}()

	start, end := initIterationForDB(dbName)
	if start == nil || end == nil {
		return nil, fmt.Errorf("%s db doesn't exist", dbName)
	}

	if err = start(ctx, cacheOpts...); err != nil {
		return nil, err
	}
	defer end(ctx)

	logger.Debug(ctx, "Querying through the entries in the db")
	for {
		entry, err := nextEntryForDB(ctx, dbName)
		if err != nil {
			// The iteration ends with an cache.ErrNoEnt, even when it's successful.
			return entries, err
		}
		entries = append(entries, entry.String())
	}
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

type stringer interface {
	String() string
}

func nextEntryForDB(ctx context.Context, dbName string) (stringer, error) {
	var entry stringer
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
