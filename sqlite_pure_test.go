//go:build !cgo
// +build !cgo

package sqlite

import (
	"database/sql"
	"database/sql/driver"
	"os"
	"testing"

	"modernc.org/sqlite"
)

const (
	// This is the DSN of the in-memory SQLite database for these tests.
	InMemoryDSN = "testdatabase"
	// This is the custom SQLite driver name.
	CustomDriverName = "my_custom_driver"
)

func TestMain(m *testing.M) {
	// Register a custom function to the default SQLite driver.
	sqlite.MustRegisterDeterministicScalarFunction("my_custom_function", -1, func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		return "my-result", nil
	})

	// Register the custom SQlite driver.
	// modernc.org/sqlite doesn't support registering functions to custom drivers.
	sql.Register(CustomDriverName,
		&sqlite.Driver{},
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
			querySuccess: true,
		},
		{
			description: "Custom driver, custom function",
			dialector: &Dialector{
				DriverName: CustomDriverName,
				DSN:        InMemoryDSN,
			},
			openSuccess:  true,
			query:        "SELECT my_custom_function()",
			querySuccess: false,
		},
	}...)

	os.Exit(m.Run())
}
