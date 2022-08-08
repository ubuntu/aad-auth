package cache

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
)

/*

Should the process of opening the file be part of the function?
Seems better if the function just print to a Writer.

dir := filepath.Join("testdata", t.Name())
os.MkdirAll(dir, os.ModePerm)
file, err := os.Create(filepath.Join(dir, "cache_dump"))
if err != nil {
	log.Fatal(err)
}
defer file.Close()

err = c.DumpData(context.Background(), file)
if err != nil {
	log.Fatal(err)
}

*/

// DumpData dumps the data of all tables from Cache into the specified output.
// If out is nil, then a cache_dump file is created and the data is written to it instead.
func (c *Cache) DumpData(ctx context.Context, out io.Writer) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("Could not dump data from cache: %v", err)
		}
	}()

	// Creates a file for output if no writer was provided.
	if out == nil {
		file, err := os.Create("cache_dump")
		if err != nil {
			return fmt.Errorf("could not create file %s: %v", "cache_dump", err)
		}
		defer file.Close()
		out = file
	}

	// Selects the table names from the database.
	query, err := c.db.Query("SELECT name FROM sqlite_schema WHERE type = 'table'")
	if err != nil {
		return fmt.Errorf("could not query tables names from cache: %v", err)
	}

	// Iterates through each table and dumps their data.
	var tableName string
	for query.Next() {
		query.Scan(&tableName)
		if err != nil {
			return fmt.Errorf("could not scan from query result: %v", err)
		}

		out.Write([]byte(tableName + "\n"))
		err = c.DumpDataFromTable(ctx, tableName, out)
		if err != nil {
			return err
		}
	}

	return nil
}

// DumpDataFromTable prints all the data contained in the specified table.
// If out is nil, then a table_dump file is created and the data is written to it instead.
func (c *Cache) DumpDataFromTable(ctx context.Context, tableName string, out io.Writer) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("Could not dump data from table %s: %v", tableName, err)
		}
	}()

	// Creates a file for output if no io.Writer was provided.
	if out == nil {
		file, err := os.Create(tableName + "_dump")
		if err != nil {
			return fmt.Errorf("could not create file %s: %v", tableName+"_dump", err)
		}
		defer file.Close()
		out = file
	}

	// Queries for all rows in the table.
	rows, err := c.db.Query(fmt.Sprintf("select * from %s", tableName))
	if err != nil {
		return fmt.Errorf("could not query from %s: %v", tableName, err)
	}

	cols, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("Could not get the db rows: %v", err)
	}

	for i, col := range cols {
		out.Write([]byte(col))
		if i < len(cols)-1 {
			out.Write([]byte(","))
		}
	}
	out.Write([]byte("\n"))

	// In order to make the function less static, the data of the row is read into a slice of bytes.
	data := make([][]byte, len(cols))
	ptr := make([]any, len(cols))
	for i := range data {
		ptr[i] = &data[i]
	}

	// Iterates through every row of the table, printing the results as csv to the io.Writer.
	for rows.Next() {
		err = rows.Scan(ptr...)
		if err != nil {
			return fmt.Errorf("Could not scan row: %v", err)
		}

		b := bytes.Join(data, []byte(","))
		_, err := out.Write(b)
		if err != nil {
			return fmt.Errorf("Could not write to file: %v", err)
		}
		out.Write([]byte("\n"))
	}

	return nil
}
