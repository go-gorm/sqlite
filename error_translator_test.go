//go:build cgo

package sqlite

import (
	"errors"
	"fmt"
	"testing"

	sqlite3 "github.com/mattn/go-sqlite3"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openTranslating(t *testing.T, dsn string) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(Open(dsn), &gorm.Config{
		Logger:         logger.Default.LogMode(logger.Silent),
		TranslateError: true,
	})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	return db
}

func TestTranslateDuplicatedKeyOnUniqueColumn(t *testing.T) {
	type Article struct {
		ArticleNumber string `gorm:"unique"`
	}

	db := openTranslating(t, "file:translate_unique?mode=memory&cache=shared")
	if err := db.AutoMigrate(&Article{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	if err := db.Create(&Article{ArticleNumber: "A00000XX"}).Error; err != nil {
		t.Fatalf("expected the first create to succeed, got: %v", err)
	}

	err := db.Create(&Article{ArticleNumber: "A00000XX"}).Error
	if !errors.Is(err, gorm.ErrDuplicatedKey) {
		t.Errorf("expected gorm.ErrDuplicatedKey, got: %v", err)
	}
}

func TestTranslateDuplicatedKeyOnPrimaryKey(t *testing.T) {
	type Product struct {
		Code string `gorm:"primaryKey"`
	}

	db := openTranslating(t, "file:translate_pk?mode=memory&cache=shared")
	if err := db.AutoMigrate(&Product{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	if err := db.Create(&Product{Code: "P1"}).Error; err != nil {
		t.Fatalf("expected the first create to succeed, got: %v", err)
	}

	err := db.Create(&Product{Code: "P1"}).Error
	if !errors.Is(err, gorm.ErrDuplicatedKey) {
		t.Errorf("expected gorm.ErrDuplicatedKey, got: %v", err)
	}
}

func TestTranslateForeignKeyViolated(t *testing.T) {
	db := openTranslating(t, "file:translate_fk?mode=memory&cache=shared&_foreign_keys=on")

	if err := db.Exec("CREATE TABLE authors (id integer primary key)").Error; err != nil {
		t.Fatalf("failed to create authors: %v", err)
	}
	if err := db.Exec("CREATE TABLE books (id integer primary key, author_id integer REFERENCES authors(id))").Error; err != nil {
		t.Fatalf("failed to create books: %v", err)
	}

	err := db.Exec("INSERT INTO books (author_id) VALUES (404)").Error
	if !errors.Is(err, gorm.ErrForeignKeyViolated) {
		t.Errorf("expected gorm.ErrForeignKeyViolated, got: %v", err)
	}
}

func TestTranslateWrappedError(t *testing.T) {
	dialector := Dialector{}
	sqliteErr := sqlite3.Error{
		Code:         sqlite3.ErrConstraint,
		ExtendedCode: sqlite3.ErrConstraintUnique,
	}

	if err := dialector.Translate(sqliteErr); !errors.Is(err, gorm.ErrDuplicatedKey) {
		t.Errorf("expected gorm.ErrDuplicatedKey, got: %v", err)
	}

	wrapped := fmt.Errorf("inserting article: %w", sqliteErr)
	if err := dialector.Translate(wrapped); !errors.Is(err, gorm.ErrDuplicatedKey) {
		t.Errorf("expected a wrapped error to be translated to gorm.ErrDuplicatedKey, got: %v", err)
	}

	if err := dialector.Translate(&sqliteErr); !errors.Is(err, gorm.ErrDuplicatedKey) {
		t.Errorf("expected a pointer error to be translated to gorm.ErrDuplicatedKey, got: %v", err)
	}
}

// errWithResultCodeFields looks like a sqlite error once it goes through JSON
// but comes from somewhere else entirely.
type errWithResultCodeFields struct {
	Code         int
	ExtendedCode int
	SystemErrno  int
}

func (errWithResultCodeFields) Error() string { return "unrelated error from another library" }

func TestTranslateLeavesForeignErrorTypesAlone(t *testing.T) {
	dialector := Dialector{}

	err := dialector.Translate(errWithResultCodeFields{Code: 19, ExtendedCode: 2067})
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		t.Error("expected an error from another package to be returned untouched")
	}
}

func TestTranslateUnmappedCode(t *testing.T) {
	dialector := Dialector{}
	sqliteErr := sqlite3.Error{
		Code:         sqlite3.ErrConstraint,
		ExtendedCode: sqlite3.ErrConstraintNotNull,
	}

	if err := dialector.Translate(sqliteErr); !errors.Is(err, sqliteErr) {
		t.Errorf("expected an unmapped code to be returned untouched, got: %v", err)
	}
}

func TestTranslateNonSQLiteError(t *testing.T) {
	dialector := Dialector{}
	plain := errors.New("some other failure")

	if err := dialector.Translate(plain); !errors.Is(err, plain) {
		t.Errorf("expected a plain error to be returned untouched, got: %v", err)
	}
}
