//go:build !cgo || (cgo && pure)
// +build !cgo cgo,pure

package sqlite

import (
	_ "modernc.org/sqlite" //sqlite driver
)

// DriverName is the default driver name for SQLite.
const DriverName = "sqlite"
