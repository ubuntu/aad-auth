package testutils

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	// Used as driver for the db.
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/cache"
)

// LoadAndUpdateFromGoldenDump loads the specified database from golden file in testdata/.
// It will update the file if the update flag is used prior to loading it.
func LoadAndUpdateFromGoldenDump(t *testing.T, ref string) map[string]Table {
	t.Helper()

	dbName := filepath.Base(ref)
	wantPath := filepath.Join(filepath.Join("testdata", t.Name()), dbName+".dump")

	if update {
		t.Logf("Updating dump file for %s", dbName)
		err := os.MkdirAll(filepath.Dir(wantPath), 0750)
		require.NoError(t, err, "could not create directory for dump files")

		f, err := os.Create(wantPath)
		require.NoError(t, err, "could not create file to dump the db")

		err = DumpDb(t, ref, f, true)
		f.Close()
		require.NoError(t, err, "could not dump the db")
	}

	f, err := os.Open(wantPath)
	require.NoError(t, err, "Wanted golden dump must be read.")
	defer f.Close()
	want, err := ReadDumpAsTables(t, f)
	require.NoError(t, err, "Could not read dump file %s", wantPath)

	return want
}

// Table is a struct that represents a table of a db.
type Table struct {
	// Cols contains the column names
	Cols []string
	// Rows is a slice where each map represents a row as row[colName]: colData
	Rows []map[string]string
}

// ReadDumpAsTables opens the file specified and reads its contents into a map[name]Table.
func ReadDumpAsTables(t *testing.T, r io.Reader) (map[string]Table, error) {
	t.Helper()

	tables := make(map[string]Table)

	data, err := io.ReadAll(r)
	require.NoError(t, err, "failed to read dump file")

	for _, table := range strings.Split(string(data), "\n\n") {
		lines := strings.Split(table, "\n")
		// Handles the last line of the dump file
		if len(lines) == 1 {
			break
		}

		require.GreaterOrEqual(t, len(lines), 3, "%q should contain 3 lines at least: name/row names/data", lines)

		// Each group of data is one table with its content.
		table := Table{}

		// Headers.
		name := lines[0]
		cols := lines[1]
		table.Cols = strings.Split(cols, ",")

		// Content.
		for _, data := range lines[2:] {
			row := make(map[string]string)
			for i, v := range strings.Split(data, ",") {
				row[table.Cols[i]] = v
			}
			table.Rows = append(table.Rows, row)
		}
		tables[name] = table
	}

	return tables, nil
}

// DumpDb opens the specified database and dumps its content into w.
func DumpDb(t *testing.T, p string, w io.Writer, usePredicatableFieldValues bool) (err error) {
	t.Helper()

	// Connects to the database
	db, err := sql.Open("sqlite3", p)
	require.NoError(t, err, "Failed to open the requested database")
	defer db.Close()

	err = dbToCsv(t, db, w, usePredicatableFieldValues)
	require.NoError(t, err, "Db should be dumped correctly")

	return nil
}

// dbDataToCsv dumps the data of all tables from the db file into the specified output.
func dbToCsv(t *testing.T, db *sql.DB, w io.Writer, usePredicatableFieldValues bool) (err error) {
	t.Helper()

	// Selects the table names from the database.
	query, err := db.Query("SELECT name FROM sqlite_schema WHERE type = 'table'")
	require.NoError(t, err, "Should be able to query the tables from the database")
	defer query.Close()

	// Iterates through each table and dumps their data.
	var tableName string
	for query.Next() {
		err = query.Scan(&tableName)
		require.NoError(t, err, "Query result should be scanned")

		_, err = w.Write([]byte(tableName + "\n"))
		require.NoError(t, err, "Failed to write table name")

		err = dumpTable(t, db, tableName, w, usePredicatableFieldValues)
		require.NoError(t, err, "Failed to dump table %s", tableName)

		_, err = w.Write([]byte("\n"))
		require.NoError(t, err, "There should be a line break after the table")
	}

	return nil
}

// dumpTable prints all the data contained in the specified table.
func dumpTable(t *testing.T, db *sql.DB, name string, w io.Writer, usePredicatableFieldValues bool) (err error) {
	t.Helper()

	// Queries for all rows in the table.
	// We can't interpolate/sanitize table names and we are in control
	// of the input in the tests.
	rows, err := db.Query(fmt.Sprintf("SELECT * FROM %s", name))
	require.NoError(t, err, "Failed to query data from %s", name)
	defer rows.Close()

	cols, err := rows.Columns()
	require.NoError(t, err, "Failed to get column names from table %s", name)

	// Prints the names of the columns as the first line of the table dump.
	_, err = w.Write([]byte(strings.Join(cols, ",") + "\n"))
	require.NoError(t, err, "Failed to write column names")

	// Initializes the structures that will be used for reading the rows values.
	data := make([]string, len(cols))
	ptr := make([]any, len(cols))
	for i := range data {
		ptr[i] = &data[i]
	}

	// Iterates through every row of the table, printing the results to w.
	for rows.Next() {
		err = rows.Scan(ptr...)
		require.NoError(t, err, "Failed to scan row")

		if usePredicatableFieldValues {
			predicatableFieldValues(name, data, cols)
		}

		// Write the entire row with its fields in CSV format.
		_, err = w.Write([]byte(strings.Join(data, ",") + "\n"))
		require.NoError(t, err, "Failed to write row")
	}

	return nil
}

// predicatableFieldValues makes changing fields, based on time, stable to compare them
// in golden files.
func predicatableFieldValues(name string, data, cols []string) {
	switch name {
	case "shadow":
		for i, col := range cols {
			if col == "password" {
				data[i] = "HASHED_PASSWORD"
				break
			}
		}

	case "passwd":
		for i, col := range cols {
			if col == "last_online_auth" {
				data[i] = "4242"
				break
			}
		}
	}
}

// PrepareDBsForTests initializes a cache in the specified directory and load it with the specified dump.
func PrepareDBsForTests(t *testing.T, cacheDir, initialCache string, options ...cache.Option) {
	t.Helper()

	// Gets the path to the testutils package.
	_, p, _, _ := runtime.Caller(0)
	testutilsPath := filepath.Dir(p)

	c := NewCacheForTests(t, cacheDir, options...)
	err := c.Close(context.Background())
	require.NoError(t, err, "Cache must be closed to enable the dump loading.")

	for _, db := range []string{"passwd.db", "shadow.db"} {
		loadDumpIntoDB(t, filepath.Join(testutilsPath, "cache_dumps", initialCache, db+".dump"), filepath.Join(cacheDir, db))
	}
}

// NewCacheForTests returns a cache that is closed automatically, with permissions set to current user.
func NewCacheForTests(t *testing.T, cacheDir string, options ...cache.Option) (c *cache.Cache) {
	t.Helper()

	uid, gid := GetCurrentUIDGID(t)
	opts := append([]cache.Option{}, cache.WithCacheDir(cacheDir),
		cache.WithRootUID(uid), cache.WithRootGID(gid), cache.WithShadowGID(gid), cache.WithTeardownDuration(0))

	opts = append(opts, options...)

	c, err := cache.New(context.Background(), opts...)
	require.NoError(t, err, "Setup: should be able to create a cache")
	t.Cleanup(func() { c.Close(context.Background()) })

	return c
}

// loadDumpIntoDB reads the specified dump file and inserts its contents into the database.
func loadDumpIntoDB(t *testing.T, dumpPath, dbPath string) {
	t.Helper()

	f, err := os.Open(dumpPath)
	require.NoError(t, err, "Expected to open dump file %s.", dumpPath)
	defer f.Close()

	dump, err := ReadDumpAsTables(t, f)
	require.NoError(t, err, "Expected to read dump file %s.", dumpPath)
	require.NoError(t, f.Close(), "File should be closed correctly.")

	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err, "Expected to open database %s.", dbPath)
	defer db.Close()

	for name, table := range dump {
		st := fmt.Sprintf("INSERT INTO %s VALUES (%s)", name, "%s")

		for _, row := range table.Rows {
			values := make([]any, len(row))
			var s string
			// Looping through the columns to ensure that the values will be ordered as supposed to.
			for i, col := range table.Cols {
				values[i] = row[col]
				if col == "last_online_auth" {
					values[i] = ParseTimeWildcard(row[col]).Unix()
				}
				s += "?,"
			}

			// Formats the statement removing the last trailing comma from the values string.
			rowSt := fmt.Sprintf(st, s[:len(s)-1])
			_, err = db.Exec(rowSt, values...)
			require.NoError(t, err, "Expected to insert %#v into the db", row)
		}
	}
}

// ParseTimeWildcard parses some time wildcards that are contained in the dump files to ensure that the loaded dbs will always present the same
// behavior when loaded for tests.
func ParseTimeWildcard(value string) time.Time {
	// c is a contant value, set to two days, that is used to ensure that the time is within some intervals.
	c := time.Hour * 48
	expirationDays := time.Duration(int64(cache.DefaultCredentialsExpiration) * 24 * int64(time.Hour))

	parsedTime := time.Now()
	var addend time.Duration
	switch value {
	case "RECENT_TIME":
		addend = -c

	case "PURGED_TIME":
		addend = -((2 * expirationDays) + c)

	case "EXPIRED_TIME":
		addend = -(expirationDays + c)

	case "FUTURE_TIME":
		addend = c
	}

	return parsedTime.Add(addend)
}
