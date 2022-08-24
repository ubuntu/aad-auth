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
		defer f.Close()

		err = DumpDb(t, ref, f, true)
		require.NoError(t, err, "could not dump the db")
	}

	want, err := ReadDumpAsTables(t, wantPath)
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
func ReadDumpAsTables(t *testing.T, p string) (map[string]Table, error) {
	t.Helper()

	f, err := os.Open(p)
	require.NoError(t, err, "failed to open dump file")
	defer f.Close()

	tables := make(map[string]Table)

	data, err := io.ReadAll(f)
	require.NoError(t, err, "failed to read dump file")

	for _, table := range strings.Split(string(data), "\n\n") {
		lines := strings.Split(table, "\n")
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
	var separateTables bool
	for query.Next() {
		if separateTables {
			_, err = w.Write([]byte("\n"))
			require.NoError(t, err, "There should be a line break between tables")
		}

		err = query.Scan(&tableName)
		require.NoError(t, err, "Query result should be scanned")

		_, err = w.Write([]byte(tableName + "\n"))
		require.NoError(t, err, "Failed to write table name")

		err = dumpTable(t, db, tableName, w, usePredicatableFieldValues)
		require.NoError(t, err, "Failed to dump table %s", tableName)

		separateTables = true
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
