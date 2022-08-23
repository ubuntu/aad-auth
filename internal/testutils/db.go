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

type optionDB struct {
	dumpName string
	dumpPath string
}

// OptionDB is a supported option to override some behaviors of some db functions.
type OptionDB func(*optionDB)

// SaveAndUpdateDump opens the specified database and saves the dump in a file.
// If the update flag is set, it also updates the dump on testdata.
func SaveAndUpdateDump(t *testing.T, dbPath string, opts ...OptionDB) {
	t.Helper()

	dbName := filepath.Base(dbPath)

	o := optionDB{
		dumpName: dbName + ".dump",
		dumpPath: filepath.Join("testdata", t.Name()),
	}

	for _, opt := range opts {
		opt(&o)
	}

	if update {
		t.Logf("Updating dump file for %s", dbName)
		err := os.MkdirAll(o.dumpPath, 0750)
		require.NoError(t, err, "could not create directory for dump files")

		f, err := os.Create(filepath.Join(o.dumpPath, o.dumpName))
		require.NoError(t, err, "could not create file to dump the db")
		defer f.Close()

		err = dumpDb(dbPath, f)
		require.NoError(t, err, "could not dump the db")
	}

	f, err := os.Create(dbPath + ".dump")
	require.NoError(t, err, "could not dump the db")
	defer f.Close()

	err = dumpDb(dbPath, f)
	require.NoError(t, err, "could not dump the db")
}

// Table is a struct that represents a table of a db.
type Table struct {
	// Cols contains the column names
	Cols []string
	// Rows is a slice where each map represents a row as row[colName]: colData
	Rows []map[string]string
}

// ReadDumpAsTables opens the file specified and reads its contents into a map[name]Table.
func ReadDumpAsTables(t *testing.T, p string) (map[string]Table, error) {
	t.Helper()

	f, err := os.Open(p)
	require.NoError(t, err, "failed to open dump file")
	defer f.Close()

	tables := make(map[string]Table)

	data, err := io.ReadAll(f)
	require.NoError(t, err, "failed to read dump file")
	if err != nil {
		return nil, err
	}
	for _, table := range strings.Split(string(data), "\n\n") {
		lines := strings.Split(table, "\n")
		if len(lines) < 3 {
			return nil, fmt.Errorf("%q should contains 3 lines at least: name/row names/data", lines)
		}

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

// dumpDb opens the specified database and dumps its content into w.
// TODO: use testing.T and require.
func dumpDb(p string, w io.Writer) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to open and dump the dbs: %w", err)
		}
	}()

	// Connects to the database
	db, err := sql.Open("sqlite3", p)
	if err != nil {
		return fmt.Errorf("failed to open the requested database: %w", err)
	}
	defer db.Close()

	if err = dbToCsv(db, w); err != nil {
		return err
	}

	return nil
}

// dbDataToCsv dumps the data of all tables from the db file into the specified output.
// TODO: use testing.T and require.
func dbToCsv(db *sql.DB, w io.Writer) (err error) {
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
	defer query.Close()

	// Iterates through each table and dumps their data.
	var tableName string
	var separateTables bool
	for query.Next() {
		if separateTables {
			if _, err = w.Write([]byte("\n")); err != nil {
				return fmt.Errorf("something went wrong when writing a line break to writer: %w", err)
			}
		}

		if err = query.Scan(&tableName); err != nil {
			return fmt.Errorf("could not scan from query result: %w", err)
		}

		if _, err = w.Write([]byte(tableName + "\n")); err != nil {
			return fmt.Errorf("something went wrong when writing to writer: %w", err)
		}

		if err = dumpTable(db, tableName, w); err != nil {
			return err
		}

		separateTables = true
	}

	return nil
}

// dumpTable prints all the data contained in the specified table.
// TODO: use testing.T and require.
func dumpTable(db *sql.DB, name string, w io.Writer) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("could not dump data from table %s: %w", name, err)
		}
	}()

	if w == nil {
		return fmt.Errorf("no writer available")
	}

	// Queries for all rows in the table.
	// We can't interpolate/sanitize table names and we are in control
	// of the input in the tests.
	rows, err := db.Query(fmt.Sprintf("SELECT * FROM %s", name))
	if err != nil {
		return fmt.Errorf("could not query from %s: %w", name, err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("could not get the db rows: %w", err)
	}

	// Prints the names of the columns as the first line of the table dump.
	if _, err = w.Write([]byte(strings.Join(cols, ",") + "\n")); err != nil {
		return fmt.Errorf("could not write columns names: %w", err)
	}

	// Initializes the structures that will be used for reading the rows values.
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

		// Write the entire row with its fields in CSV format.
		if _, err = w.Write([]byte(strings.Join(data, ",") + "\n")); err != nil {
			return fmt.Errorf("could not write row: %w", err)
		}
	}

	return nil
}
