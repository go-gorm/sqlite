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

// DropColumn and AlterColumn must accept a table-name string like the base
// GORM migrator and the other dialects do; stmt.Schema is nil in that case
// and used to cause a nil pointer dereference.
func TestMigratorStringTableName(t *testing.T) {
	db, err := gorm.Open(Open("file:migrator_string_value?mode=memory&cache=shared"), &gorm.Config{})
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
	if err := db.Exec("CREATE TABLE `string_value_table` (`id` integer, `b` text)").Error; err != nil {
		t.Fatal(err)
	}

	if err := db.Migrator().DropColumn("string_value_table", "b"); err != nil {
		t.Errorf("DropColumn with a table-name string: %v", err)
	}
	if db.Migrator().HasColumn("string_value_table", "b") {
		t.Error("column b still present after DropColumn")
	}

	// AlterColumn needs the model schema to build the new column type; a
	// table-name string must produce a regular error, not a panic.
	if err := db.Migrator().AlterColumn("string_value_table", "id"); err == nil {
		t.Error("AlterColumn with a table-name string: expected an error, got nil")
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

type ConstraintParent struct {
	ID int `gorm:"primaryKey"`
}

func (ConstraintParent) TableName() string { return "constraint_parents" }

type ConstraintChild struct {
	ID       int
	ParentID int
	Parent   ConstraintParent `gorm:"foreignKey:ParentID"`
}

func (ConstraintChild) TableName() string { return "constraint_children" }

// CreateConstraint appends the constraint to the field list as a `CONSTRAINT ?
// FOREIGN KEY ...` placeholder clause; getColumns must not mistake it for a
// column, or the data copy fails with "no column named CONSTRAINT".
func TestCreateConstraintPlaceholder(t *testing.T) {
	db, err := gorm.Open(Open("file:constraint_placeholder?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	if err := db.AutoMigrate(&ConstraintParent{}); err != nil {
		t.Fatal(err)
	}
	// existing table missing the foreign key declared in the model
	if err := db.Exec("CREATE TABLE `constraint_children` (`id` integer, `parent_id` integer)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO `constraint_children`(`id`,`parent_id`) VALUES (1, NULL)").Error; err != nil {
		t.Fatal(err)
	}

	if err := db.AutoMigrate(&ConstraintChild{}); err != nil {
		t.Fatalf("AutoMigrate adding the missing foreign key: %v", err)
	}

	var n int
	if err := db.Raw("SELECT count(*) FROM `constraint_children`").Scan(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("rows after rebuild = %d, want 1", n)
	}
	if !db.Migrator().HasConstraint(&ConstraintChild{}, "fk_constraint_children_parent") {
		t.Error("foreign key constraint missing after AutoMigrate")
	}
}

type compositeKeyModel struct {
	A int    `gorm:"primaryKey;autoIncrement"`
	B string `gorm:"primaryKey"`
}

func (compositeKeyModel) TableName() string { return "composite_key_models" }

// A composite primary key that includes an auto-increment field must keep all
// key columns. AUTOINCREMENT only applies to a single-column INTEGER PRIMARY
// KEY, and emitting it made GORM skip the table-level PRIMARY KEY clause,
// silently reducing the key to one column.
func TestCompositePrimaryKeyAutoIncrement(t *testing.T) {
	db, err := gorm.Open(Open("file:composite_pk?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	if err := db.AutoMigrate(&compositeKeyModel{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	var pkCount int
	if err := db.Raw("SELECT count(*) FROM pragma_table_info('composite_key_models') WHERE pk > 0").Scan(&pkCount).Error; err != nil {
		t.Fatal(err)
	}
	if pkCount != 2 {
		var ddl string
		_ = db.Raw("SELECT sql FROM sqlite_master WHERE type='table' AND name='composite_key_models'").Scan(&ddl).Error
		t.Errorf("primary key column count = %d, want 2 (DDL: %s)", pkCount, ddl)
	}
}

// GetTables must not report SQLite internal tables such as sqlite_sequence,
// which appears as soon as any table uses AUTOINCREMENT.
func TestGetTablesExcludesInternal(t *testing.T) {
	db, err := gorm.Open(Open("file:internal_tables?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	if err := db.Exec("CREATE TABLE `seq_table` (`id` integer PRIMARY KEY AUTOINCREMENT)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO `seq_table` DEFAULT VALUES").Error; err != nil {
		t.Fatal(err)
	}

	tables, err := db.Migrator().GetTables()
	if err != nil {
		t.Fatal(err)
	}
	for _, tb := range tables {
		if strings.HasPrefix(tb, "sqlite_") {
			t.Errorf("GetTables returned internal table %q", tb)
		}
	}
	if len(tables) != 1 || tables[0] != "seq_table" {
		t.Errorf("GetTables = %v, want [seq_table]", tables)
	}
}

type checkedModel struct {
	ID  int
	Age int `gorm:"check:age_positive,age > 0"`
}

func (checkedModel) TableName() string { return "checked_models" }

// HasConstraint must match the exact constraint name instead of a LIKE
// substring, and DropColumn must fail for a column that is not in the DDL
// instead of silently rebuilding an identical table.
func TestConstraintAndColumnMatching(t *testing.T) {
	db, err := gorm.Open(Open("file:constraint_matching?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	if err := db.AutoMigrate(&checkedModel{}); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasConstraint(&checkedModel{}, "age_positive") {
		t.Error("HasConstraint(age_positive) = false, want true")
	}
	if db.Migrator().HasConstraint(&checkedModel{}, "age_pos") {
		t.Error("HasConstraint(age_pos) matched by prefix, want false")
	}

	// unquoted DDL: the column must still be dropped
	if err := db.Exec("CREATE TABLE plain_cols (id integer, name text)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropColumn(&plainColModel{}, "name"); err != nil {
		t.Fatalf("DropColumn on unquoted DDL: %v", err)
	}
	if db.Migrator().HasColumn(&plainColModel{}, "name") {
		t.Error("column still present after DropColumn on unquoted DDL")
	}
	// a missing column must be an error, not a silent no-op
	if err := db.Migrator().DropColumn(&plainColModel{}, "missing"); err == nil {
		t.Error("DropColumn(missing) = nil, want error")
	}
	// a missing table must be reported clearly
	if err := db.Migrator().DropColumn(&noSuchTableModel{}, "col"); err == nil || !strings.Contains(err.Error(), "table not found") {
		t.Errorf("DropColumn on a missing table = %v, want a table-not-found error", err)
	}
}

type plainColModel struct {
	ID   int
	Name string
}

func (plainColModel) TableName() string { return "plain_cols" }

type noSuchTableModel struct {
	ID int
}

func (noSuchTableModel) TableName() string { return "no_such_table" }
