//go:build cgo

package sqlite

import (
	"errors"

	sqlite3 "github.com/mattn/go-sqlite3"
)

// Translate it will translate the error to native gorm errors.
func (dialector Dialector) Translate(err error) error {
	code, ok := extendedResultCode(err)
	if !ok {
		return err
	}

	if translatedErr, found := errCodes[code]; found {
		return translatedErr
	}
	return err
}

// extendedResultCode reports the sqlite extended result code carried by err, if
// any. The error chain is walked so that an error wrapped by a callback or a
// plugin is still recognised.
func extendedResultCode(err error) (int, bool) {
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		return int(sqliteErr.ExtendedCode), true
	}

	var sqliteErrPtr *sqlite3.Error
	if errors.As(err, &sqliteErrPtr) && sqliteErrPtr != nil {
		return int(sqliteErrPtr.ExtendedCode), true
	}

	return 0, false
}
