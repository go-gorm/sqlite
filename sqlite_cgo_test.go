//go:build cgo
// +build cgo

package sqlite

import (
	"database/sql"
	"os"
	"testing"

	"github.com/mattn/go-sqlite3"
)

const (
	// This is the DSN of the in-memory SQLite database for these tests.
	InMemoryDSN = "file:testdatabase?mode=memory&cache=shared"
	// This is the custom SQLite driver name.
	CustomDriverName = "my_custom_driver"
)

func TestMain(m *testing.M) {
	// Register the custom SQlite3 driver.
	// It will have one custom function called "my_custom_function".
	sql.Register(CustomDriverName,
		// This is the custom SQLite driver.
		&sqlite3.SQLiteDriver{
			ConnectHook: func(conn *sqlite3.SQLiteConn) error {
				// Define the `concat` function, since we use this elsewhere.
				err := conn.RegisterFunc(
					"my_custom_function",
					func(arguments ...interface{}) (string, error) {
						return "my-result", nil // Return a string value.
					},
					true,
				)
				return err
			},
		},
	)

	rows = append(rows, []testRow{
		{
			description: "Explicit default driver, custom function",
			dialector: &Dialector{
				DriverName: DriverName,
				DSN:        InMemoryDSN,
			},
			openSuccess:  true,
			query:        "SELECT my_custom_function()",
			querySuccess: false,
		},
		{
			description: "Custom driver, custom function",
			dialector: &Dialector{
				DriverName: CustomDriverName,
				DSN:        InMemoryDSN,
			},
			openSuccess:  true,
			query:        "SELECT my_custom_function()",
			querySuccess: true,
		},
	}...)

	os.Exit(m.Run())
}
