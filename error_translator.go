package sqlite

import (
	"gorm.io/gorm"
)

// The error codes to map sqlite errors to gorm errors, here is a reference about error codes for sqlite https://www.sqlite.org/rescode.html.
var errCodes = map[int]error{
	1555: gorm.ErrDuplicatedKey,
	2067: gorm.ErrDuplicatedKey,
	787:  gorm.ErrForeignKeyViolated,
}

// ErrMessage was the intermediate shape the extended result code used to be
// decoded into.
//
// Deprecated: Translate reads the extended result code from sqlite3.Error
// directly and no longer decodes errors through JSON. This type is retained so
// that code referring to it keeps compiling.
type ErrMessage struct {
	Code         int `json:"Code"`
	ExtendedCode int `json:"ExtendedCode"`
	SystemErrno  int `json:"SystemErrno"`
}
