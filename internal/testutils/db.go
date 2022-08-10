package testutils

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	// Used as driver for the db.
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

type dbOption struct {
	dumpName string
	dumpPath string
}

// DbOption is a supported option to override some behaviors of some db functions.
type DbOption func(*dbOption)

// WithDumpPath overrides the default path used to save the dump files.
func WithDumpPath(path string) DbOption {
	return func(o *dbOption) {
		o.dumpPath = path
	}
}

// WithDumpName overrides the default name used to save the dump files.
func WithDumpName(name string) DbOption {
	return func(o *dbOption) {
		o.dumpName = name
	}
}

// SaveAndLoadFromDump ...
func SaveAndLoadFromDump(t *testing.T, dbPath, dump string, opts ...DbOption) string {
	t.Helper()

	dbName := dbPath[strings.LastIndex(dbPath, "/")+1:]

	o := dbOption{
		dumpName: dbName + "_dump",
		dumpPath: filepath.Join("testdata", t.Name()),
	}

	for _, opt := range opts {
		opt(&o)
	}

	if update {
		t.Logf("Updating dump file for %s", dbPath)
		err := os.MkdirAll(o.dumpPath, 0755)
		require.NoError(t, err, "could not create directory for dump files")

		f, err := os.Create(filepath.Join(o.dumpPath, o.dumpName))
		require.NoError(t, err, "could not create file to dump the db")
		defer f.Close()

		_, err = f.Write([]byte(dump))
		require.NoError(t, err, "Cannot update the dump file for %s", dbName)
	}

	dump, err := ReadDump(filepath.Join(o.dumpPath, o.dumpName))
	require.NoError(t, err, "Failed to read the dump file")

	return dump
}

// ReadDump reads the specified dump file and returns a string with its contents.
func ReadDump(dumpPath string) (string, error) {
	f, err := os.ReadFile(dumpPath)
	if err != nil {
		return "", fmt.Errorf("failed to read dump file: %w", err)
	}
	return string(f), err
}

// OpenAndDumpDb opens the specified database and dumps its content into w.
func OpenAndDumpDb(dbPath string, w io.Writer) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to open and dump the dbs: %w", err)
		}
	}()

	// Connects to the database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open the requested database: %w", err)
	}
	defer db.Close()

	if err = dbDataToCsv(db, w); err != nil {
		return err
	}

	return nil
}

// dbDataToCsv dumps the data of all tables from the db file into the specified output.
// If w is nil, an error is returned.
func dbDataToCsv(db *sql.DB, w io.Writer) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("could not dump data from db: %w", err)
		}
	}()

	if w == nil {
		return fmt.Errorf("no writer available")
	}

	if db == nil {
		return fmt.Errorf("no database available")
	}

	// Selects the table names from the database.
	query, err := db.Query("SELECT name FROM sqlite_schema WHERE type = 'table'")
	if err != nil {
		return fmt.Errorf("could not query tables names from db: %w", err)
	}

	// Iterates through each table and dumps their data.
	var tableName string
	for query.Next() {
		if err = query.Scan(&tableName); err != nil {
			return fmt.Errorf("could not scan from query result: %w", err)
		}

		if _, err = w.Write([]byte(tableName + "\n")); err != nil {
			return fmt.Errorf("something went wrong when writing to writer: %w", err)
		}

		if err = dumpDataFromTable(db, tableName, w); err != nil {
			return err
		}
	}

	return nil
}

// dumpDataFromTable prints all the data contained in the specified table.
// If w is nil, an error is returned.
func dumpDataFromTable(db *sql.DB, tableName string, w io.Writer) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("could not dump data from table %s: %w", tableName, err)
		}
	}()

	if w == nil {
		return fmt.Errorf("no writer available")
	}

	// Queries for all rows in the table.
	rows, err := db.Query(fmt.Sprintf("SELECT * FROM %s", tableName))
	if err != nil {
		return fmt.Errorf("could not query from %s: %w", tableName, err)
	}

	cols, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("could not get the db rows: %w", err)
	}

	// Prints the names of the columns as the first line of the table dump
	if _, err = w.Write([]byte(strings.Join(cols, ",") + "\n")); err != nil {
		return fmt.Errorf("could not write columns names: %w", err)
	}

	// Initializes the structures that will be used for reading the rows values
	data := make([]string, len(cols))
	ptr := make([]any, len(cols))
	for i := range data {
		ptr[i] = &data[i]
	}

	// Iterates through every row of the table, printing the results to w.
	for rows.Next() {
		if err = rows.Scan(ptr...); err != nil {
			return fmt.Errorf("could not scan row: %w", err)
		}

		// Write the entire row with its fields in CSV format
		if _, err = w.Write([]byte(strings.Join(data, ",") + "\n")); err != nil {
			return fmt.Errorf("could not write row: %w", err)
		}
	}

	return nil
}
