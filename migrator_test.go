package sqlite

import (
	"strings"
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

type RebuildIdxTable struct {
	ID int
	A  string
	B  string
}

func (RebuildIdxTable) TableName() string { return "rebuild_idx_table" }

// recreateTable drops the old table together with its indexes and triggers;
// they must be recreated on the rebuilt table. Indexes on a dropped column
// are the exception: they can no longer apply.
func TestRecreateTablePreservesIndexesAndTriggers(t *testing.T) {
	db := openRecreateTestDB(t, "recreate_idx")
	stmts := []string{
		"CREATE TABLE `rebuild_idx_table` (`id` integer, `a` text, `b` text)",
		"CREATE INDEX `idx_keep` ON `rebuild_idx_table`(`a`)",
		"CREATE INDEX `idx_gone` ON `rebuild_idx_table`(`b`)",
		"CREATE TABLE `rebuild_audit` (`msg` text)",
		"CREATE TRIGGER `trg_keep` AFTER INSERT ON `rebuild_idx_table` BEGIN INSERT INTO `rebuild_audit`(`msg`) VALUES ('x'); END",
	}
	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			t.Fatal(err)
		}
	}

	if err := db.Migrator().DropColumn(&RebuildIdxTable{}, "b"); err != nil {
		t.Fatalf("DropColumn: %v", err)
	}

	var names []string
	if err := db.Raw("SELECT name FROM sqlite_master WHERE tbl_name = 'rebuild_idx_table' AND type IN ('index','trigger')").Scan(&names).Error; err != nil {
		t.Fatalf("querying sqlite_master: %v", err)
	}
	got := strings.Join(names, ",")
	if !strings.Contains(got, "idx_keep") {
		t.Errorf("index idx_keep lost after DropColumn, remaining: %v", names)
	}
	if !strings.Contains(got, "trg_keep") {
		t.Errorf("trigger trg_keep lost after DropColumn, remaining: %v", names)
	}
	if strings.Contains(got, "idx_gone") {
		t.Errorf("index idx_gone references the dropped column and must not survive, remaining: %v", names)
	}

	// the trigger still works on the rebuilt table
	if err := db.Exec("INSERT INTO `rebuild_idx_table`(`id`,`a`) VALUES (1,'v')").Error; err != nil {
		t.Fatal(err)
	}
	var auditCount int
	if err := db.Raw("SELECT count(*) FROM rebuild_audit").Scan(&auditCount).Error; err != nil {
		t.Fatalf("querying rebuild_audit: %v", err)
	}
	if auditCount != 1 {
		t.Errorf("trigger did not fire after rebuild, audit rows = %d", auditCount)
	}
}

type RebuildOptsTable struct {
	ID string `gorm:"primaryKey"`
	A  string
	B  string
}

func (RebuildOptsTable) TableName() string { return "rebuild_opts_table" }

// Table options after the column list (WITHOUT ROWID, STRICT) must survive a
// table rebuild instead of silently turning the table into a plain rowid one.
func TestRecreateTablePreservesTableOptions(t *testing.T) {
	db := openRecreateTestDB(t, "recreate_opts")
	if err := db.Exec("CREATE TABLE `rebuild_opts_table` (`id` text PRIMARY KEY, `a` text, `b` text) WITHOUT ROWID").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropColumn(&RebuildOptsTable{}, "b"); err != nil {
		t.Fatalf("DropColumn: %v", err)
	}

	var ddl string
	if err := db.Raw("SELECT sql FROM sqlite_master WHERE type='table' AND name='rebuild_opts_table'").Scan(&ddl).Error; err != nil {
		t.Fatalf("querying sqlite_master: %v", err)
	}
	if !strings.Contains(ddl, "WITHOUT ROWID") {
		t.Errorf("WITHOUT ROWID lost after rebuild: %s", ddl)
	}
}

type RebuildViewedTable struct {
	ID int
	A  string
	B  string
}

func (RebuildViewedTable) TableName() string { return "rebuild_viewed" }

// Rebuilding a table that a view references used to fail with "error in view
// ...: no such table" because the RENAME step re-resolves the view while the
// original table name doesn't exist yet (issue #225).
func TestRecreateTableWithView(t *testing.T) {
	db := openRecreateTestDB(t, "recreate_view")
	if err := db.Exec("CREATE TABLE `rebuild_viewed` (`id` integer, `a` text, `b` text)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE VIEW `rebuild_viewed_v` AS SELECT `a` FROM `rebuild_viewed`").Error; err != nil {
		t.Fatal(err)
	}

	if err := db.Migrator().DropColumn(&RebuildViewedTable{}, "b"); err != nil {
		t.Fatalf("DropColumn with a referencing view: %v", err)
	}

	var n int
	if err := db.Raw("SELECT count(*) FROM `rebuild_viewed_v`").Scan(&n).Error; err != nil {
		t.Errorf("view is broken after rebuild: %v", err)
	}
}

func openRecreateTestDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
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
	return db
}
