//go:build !cgo
// +build !cgo

package sqlite

import (
	_ "modernc.org/sqlite" //sqlite driver
)

// DriverName is the default driver name for SQLite.
const DriverName = "sqlite"
