package sqlite

import (
	"testing"

	"gorm.io/gorm"
)

// HasColumn used to match the column name as a LIKE substring against the
// table DDL, which reported columns as present when the name merely appeared
// inside another column name or a string default value. AutoMigrate then
// skipped AddColumn and later queries failed with "no such column".
func TestHasColumnNoFalsePositive(t *testing.T) {
	db, err := gorm.Open(Open("file:hascolumn_exact?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	// close the pool so the shared in-memory database is torn down and
	// repeated runs (go test -count=N) start from a clean state
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	// name is a substring of first_name in an unquoted DDL
	if err := db.Exec("CREATE TABLE plaincols (id integer, first_name text)").Error; err != nil {
		t.Fatal(err)
	}
	if db.Migrator().HasColumn("plaincols", "name") {
		t.Error(`HasColumn("plaincols", "name") = true, want false (only first_name exists)`)
	}
	if !db.Migrator().HasColumn("plaincols", "first_name") {
		t.Error(`HasColumn("plaincols", "first_name") = false, want true`)
	}

	// name appears inside a string default value
	if err := db.Exec("CREATE TABLE `defv` (`id` integer, `cfg` text DEFAULT 'name value')").Error; err != nil {
		t.Fatal(err)
	}
	if db.Migrator().HasColumn("defv", "name") {
		t.Error(`HasColumn("defv", "name") = true, want false (name only appears in a default value)`)
	}
	if !db.Migrator().HasColumn("defv", "cfg") {
		t.Error(`HasColumn("defv", "cfg") = false, want true`)
	}
}
