//go:build integrationtests

// This tag is only used for integration tests. It allows to safeguard the cache
// dump, which can contain sensitive information.

package cache

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// DumpData dumps the data of all tables from Cache into the specified output.
// If w is nil, an error is returned.
func (c *Cache) DumpData(ctx context.Context, w io.Writer) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("could not dump data from cache: %w", err)
		}
	}()

	if w == nil {
		return fmt.Errorf("nil writer")
	}

	// Selects the table names from the database.
	query, err := c.db.Query("SELECT name FROM sqlite_schema WHERE type = 'table'")
	if err != nil {
		return fmt.Errorf("could not query tables names from cache: %w", err)
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

		if err = c.DumpDataFromTable(ctx, tableName, w); err != nil {
			return err
		}
	}

	return nil
}

// DumpDataFromTable prints all the data contained in the specified table.
// If w is nil, an error is returned.
func (c *Cache) DumpDataFromTable(ctx context.Context, tableName string, w io.Writer) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("could not dump data from table %s: %w", tableName, err)
		}
	}()

	if w == nil {
		return fmt.Errorf("nil writer")
	}

	// Queries for all rows in the table.
	rows, err := c.db.Query(fmt.Sprintf("select * from %s", tableName))
	if err != nil {
		return fmt.Errorf("could not query from %s: %w", tableName, err)
	}

	cols, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("could not get the db rows: %w", err)
	}

	// Prints the names of the columns as the first line of the table dump
	if _, err = w.Write([]byte(strings.Join(cols, ",") + "\n")); err != nil {
		return fmt.Errorf("could not write columns names to file: %w", err)
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

		// Joins the strings in data into a single string with a newline and writes it to w
		if _, err = w.Write([]byte(strings.Join(data, ",") + "\n")); err != nil {
			return fmt.Errorf("could not write to file: %w", err)
		}
	}

	return nil
}
