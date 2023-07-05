//go:build cgo && !pure
// +build cgo,!pure

package sqlite

import (
	_ "github.com/mattn/go-sqlite3" //sqlite driver
)

// DriverName is the default driver name for SQLite.
const DriverName = "sqlite3"
