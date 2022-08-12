package testutils

import (
	"bufio"
	"database/sql"
	"errors"
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

// WithDumpPath overrides the default path used to save the dump files.
func WithDumpPath(path string) OptionDB {
	return func(o *optionDB) {
		o.dumpPath = path
	}
}

// WithDumpName overrides the default name used to save the dump files.
func WithDumpName(name string) OptionDB {
	return func(o *optionDB) {
		o.dumpName = name
	}
}

// SaveAndUpdateDump opens the specified database and saves the dump in a file.
// If the update flag is set, it also updates the dump on testdata.
func SaveAndUpdateDump(t *testing.T, dbPath string, opts ...OptionDB) {
	t.Helper()

	sep := strings.LastIndex(dbPath, "/")
	dbName := dbPath[sep+1:]

	o := optionDB{
		dumpName: dbName + "_dump",
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

		err = openAndDumpDb(dbPath, f)
		require.NoError(t, err, "could not dump the db")
	}

	f, err := os.Create(dbPath + "_dump")
	require.NoError(t, err, "could not dump the db")
	defer f.Close()

	err = openAndDumpDb(dbPath, f)
	require.NoError(t, err, "could not dump the db")
}

// ReadDumpAsString opens the file specified and reads its contents into a string.
func ReadDumpAsString(dumpPath string) (string, error) {
	f, err := os.ReadFile(dumpPath)
	if err != nil {
		return "", fmt.Errorf("failed to read dump file: %w", err)
	}
	return string(f), err
}

// Table is a struct that represents a table of a db.
type Table struct {
	// Contains the column names
	Cols []string
	// Contains the column types as Types[colName]: colType
	Types map[string]string
	// Each map represents a row as row[colName]: colData
	Rows []map[string]string
}

// ReadDumpAsTables opens the file specified and reads its contents into a []Table.
func ReadDumpAsTables(dumpPath string) (map[string]Table, error) {
	f, err := os.Open(dumpPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open dum file: %w", err)
	}
	defer f.Close()

	tables := make(map[string]Table)

	buf := bufio.NewReader(f)
	for {
		if buf.Size() == 1 {
			break
		}
		name, _ := buf.ReadString('\n')
		if name == "" {
			break
		}

		name = name[:len(name)-1]
		tables[name], err = readTableFromBuffer(buf)
		if !errors.Is(err, io.EOF) {
			return nil, err
		}
	}

	return tables, nil
}

func readTableFromBuffer(b *bufio.Reader) (Table, error) {
	table := Table{}

	// Reads column names
	line, err := b.ReadString('\n')
	if err != nil {
		return Table{}, err
	}
	table.Cols = strings.Split(line[:len(line)-1], ",")

	// Reads column types
	line, err = b.ReadString('\n')
	if err != nil {
		return Table{}, err
	}
	line = line[:len(line)-1]

	table.Types = make(map[string]string)
	for i, t := range strings.Split(line, ",") {
		table.Types[table.Cols[i]] = t
	}

	// Reads table data
	for {
		line, _ = b.ReadString('\n')

		// Reads until an empty line
		if line == "\n" {
			break
		}
		line = line[:len(line)-1]

		row := make(map[string]string)
		for i, v := range strings.Split(line, ",") {
			row[table.Cols[i]] = v
		}
		table.Rows = append(table.Rows, row)
	}
	return table, nil
}

// openAndDumpDb opens the specified database and dumps its content into w.
func openAndDumpDb(dbPath string, w io.Writer) (err error) {
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
// If db or w is nil, an error is returned.
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
	defer query.Close()

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

		if _, err = w.Write([]byte("\n")); err != nil {
			return fmt.Errorf("something went wrong when writing a line break to writer: %w", err)
		}
	}

	return nil
}

// dumpDataFromTable prints all the data contained in the specified table.
// If db or w is nil, an error is returned.
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
	defer rows.Close()

	ct, err := rows.ColumnTypes()
	if err != nil {
		return fmt.Errorf("could not get the db rows: %w", err)
	}
	var (
		colNames []string
		colTypes []string
	)
	for _, c := range ct {
		colNames = append(colNames, c.Name())
		colTypes = append(colTypes, c.DatabaseTypeName())
	}

	// Prints the names of the columns as the first line of the table dump.
	if _, err = w.Write([]byte(strings.Join(colNames, ",") + "\n")); err != nil {
		return fmt.Errorf("could not write columns names: %w", err)
	}

	// Prints the types of the columns as the second line of the table dump.
	if _, err = w.Write([]byte(strings.Join(colTypes, ",") + "\n")); err != nil {
		return fmt.Errorf("could not write columns types: %w", err)
	}

	// Initializes the structures that will be used for reading the rows values.
	data := make([]string, len(colNames))
	ptr := make([]any, len(colNames))
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
