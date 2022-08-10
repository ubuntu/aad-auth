package testutils

import (
	"database/sql"
	"fmt"
	"io"
	"strings"
	"testing"

	// Used as driver for the db
	_ "github.com/mattn/go-sqlite3"
)

// DbDataToCsv dumps the data of all tables from the db file into the specified output.
// If w is nil, an error is returned.
func DbDataToCsv(t *testing.T, dbPath string, w io.Writer) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("could not dump data from db: %w", err)
		}
	}()

	if w == nil {
		return fmt.Errorf("no writer available")
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("could not open db: %w", err)
	}
	defer db.Close()

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

		if err = dumpDataFromTable(t, db, tableName, w); err != nil {
			return err
		}
	}

	return nil
}

// dumpDataFromTable prints all the data contained in the specified table.
// If w is nil, an error is returned.
func dumpDataFromTable(t *testing.T, db *sql.DB, tableName string, w io.Writer) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("could not dump data from table %s: %w", tableName, err)
		}
	}()

	if w == nil {
		return fmt.Errorf("no writer available")
	}

	// Queries for all rows in the table.
	rows, err := db.Query("SELECT * FROM ?", tableName)
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
