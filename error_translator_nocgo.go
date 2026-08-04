//go:build !cgo

package sqlite

// Translate returns err unchanged.
//
// sqlite3.Error is declared in a file that imports C, so it only exists when
// cgo is enabled. Without cgo the driver cannot open a connection either, so no
// sqlite error ever reaches this function and there is nothing to translate.
func (dialector Dialector) Translate(err error) error {
	return err
}
