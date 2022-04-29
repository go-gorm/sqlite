//go:build purego
// +build purego

package sqlite

import (
	_ "modernc.org/sqlite"
)

// DriverName is the default driver name for SQLite.
const DriverName = "sqlite"
