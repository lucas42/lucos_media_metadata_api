package main

import (
	"os"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"github.com/jmoiron/sqlx"
)

func TestDatabaseWALMode(test *testing.T) {
	dbpath := "testwalmode.sqlite"
	os.Remove(dbpath)
	db := DBInit(dbpath, MockLoganne{})
	var journalMode string
	db.DB.Get(&journalMode, "PRAGMA journal_mode")
	assertEqual(test, "journal mode", "wal", journalMode)
	os.Remove(dbpath)
}

func TestDatabaseSetup(test *testing.T) {
	dbpath := "testdb.sqlite"
	os.Remove(dbpath)
	db := DBInit(dbpath, MockLoganne{})
	for _, table := range []string{"track", "predicate", "tag", "collection", "collection_track", "album", "artist", "schema_migrations"} {
		if !db.TableExists(table) {
			test.Errorf("table %q not created", table)
		}
	}
	if db.TableExists("global") {
		test.Error("Unused `global` table created")
	}
	if db.TableExists("moo-moo head") {
		test.Error("Unexpected table created")
	}
	os.Remove(dbpath)
}

func TestFreshDatabaseAllowsMultipleTagValues(test *testing.T) {
	dbpath := "testfresh.sqlite"
	os.Remove(dbpath)
	datastore := DBInit(dbpath, MockLoganne{})

	datastore.DB.MustExec(`INSERT INTO track(id, url, fingerprint, duration) VALUES(1, 'http://example.com/t1', 'fp1', 100)`)
	datastore.DB.MustExec(`INSERT INTO predicate(id) VALUES('composer')`)
	datastore.DB.MustExec(`INSERT INTO tag(trackid, predicateid, value) VALUES(1, 'composer', 'Bach')`)
	datastore.DB.MustExec(`INSERT INTO tag(trackid, predicateid, value) VALUES(1, 'composer', 'Mozart')`)

	var count int
	datastore.DB.Get(&count, "SELECT COUNT(*) FROM tag WHERE trackid = 1 AND predicateid = 'composer'")
	assertEqual(test, "multiple composer tags allowed", 2, count)

	os.Remove(dbpath)
}

func TestUpdateTagDeletesAndInserts(test *testing.T) {
	dbpath := "testupdatetag.sqlite"
	os.Remove(dbpath)
	datastore := DBInit(dbpath, MockLoganne{})

	datastore.DB.MustExec(`INSERT INTO track(id, url, fingerprint, duration) VALUES(1, 'http://example.com/t1', 'fp1', 100)`)
	datastore.DB.MustExec(`INSERT INTO predicate(id) VALUES('title')`)
	datastore.DB.MustExec(`INSERT INTO tag(trackid, predicateid, value) VALUES(1, 'title', 'Old Title')`)

	err := datastore.updateTag(1, "title", "New Title")
	if err != nil {
		test.Fatalf("updateTag failed: %v", err)
	}

	value, err := datastore.getTagValue(1, "title")
	if err != nil {
		test.Fatalf("getTagValue failed: %v", err)
	}
	assertEqual(test, "tag should be updated", "New Title", value)

	// Verify only one row exists (not duplicated)
	var count int
	datastore.DB.Get(&count, "SELECT COUNT(*) FROM tag WHERE trackid = 1 AND predicateid = 'title'")
	assertEqual(test, "should have exactly one row", 1, count)

	os.Remove(dbpath)
}

func TestUpdateTagSplitsCSVForMultiValuePredicate(test *testing.T) {
	dbpath := "testupdatetagmulti.sqlite"
	os.Remove(dbpath)
	datastore := DBInit(dbpath, MockLoganne{})

	datastore.DB.MustExec(`INSERT INTO track(id, url, fingerprint, duration) VALUES(1, 'http://example.com/t1', 'fp1', 100)`)
	datastore.DB.MustExec(`INSERT INTO predicate(id) VALUES('language')`)

	err := datastore.updateTag(1, "language", "en,fr,de")
	if err != nil {
		test.Fatalf("updateTag failed: %v", err)
	}

	// Should have 3 separate rows, not one CSV row
	var rowCount int
	datastore.DB.Get(&rowCount, "SELECT COUNT(*) FROM tag WHERE trackid = 1 AND predicateid = 'language'")
	assertEqual(test, "should have 3 separate rows", 3, rowCount)

	// Verify individual values
	var values []string
	datastore.DB.Select(&values, "SELECT value FROM tag WHERE trackid = 1 AND predicateid = 'language' ORDER BY rowid")
	if len(values) != 3 {
		test.Fatalf("Expected 3 values, got %d", len(values))
	}
	assertEqual(test, "first value", "en", values[0])
	assertEqual(test, "second value", "fr", values[1])
	assertEqual(test, "third value", "de", values[2])

	os.Remove(dbpath)
}

func TestUpdateTagDoesNotSplitCSVForSingleValuePredicate(test *testing.T) {
	dbpath := "testupdatetagsingle.sqlite"
	os.Remove(dbpath)
	datastore := DBInit(dbpath, MockLoganne{})

	datastore.DB.MustExec(`INSERT INTO track(id, url, fingerprint, duration) VALUES(1, 'http://example.com/t1', 'fp1', 100)`)
	datastore.DB.MustExec(`INSERT INTO predicate(id) VALUES('title')`)

	// title is not a multi-value predicate, so commas should be preserved as-is
	err := datastore.updateTag(1, "title", "Hello, World")
	if err != nil {
		test.Fatalf("updateTag failed: %v", err)
	}

	var rowCount int
	datastore.DB.Get(&rowCount, "SELECT COUNT(*) FROM tag WHERE trackid = 1 AND predicateid = 'title'")
	assertEqual(test, "should have exactly one row", 1, rowCount)

	value, err := datastore.getTagValue(1, "title")
	if err != nil {
		test.Fatalf("getTagValue failed: %v", err)
	}
	assertEqual(test, "value should include comma", "Hello, World", value)

	os.Remove(dbpath)
}

func TestUpdateTagSplitsCSVTrimsWhitespace(test *testing.T) {
	dbpath := "testupdatetagtrim.sqlite"
	os.Remove(dbpath)
	datastore := DBInit(dbpath, MockLoganne{})

	datastore.DB.MustExec(`INSERT INTO track(id, url, fingerprint, duration) VALUES(1, 'http://example.com/t1', 'fp1', 100)`)
	datastore.DB.MustExec(`INSERT INTO predicate(id) VALUES('mentions')`)

	err := datastore.updateTag(1, "mentions", " alice , bob , ")
	if err != nil {
		test.Fatalf("updateTag failed: %v", err)
	}

	var values []string
	datastore.DB.Select(&values, "SELECT value FROM tag WHERE trackid = 1 AND predicateid = 'mentions' ORDER BY rowid")
	if len(values) != 2 {
		test.Fatalf("Expected 2 values, got %d: %v", len(values), values)
	}
	assertEqual(test, "first value trimmed", "alice", values[0])
	assertEqual(test, "second value trimmed", "bob", values[1])

	os.Remove(dbpath)
}

// ─── Migration Runner Tests ───────────────────────────────────────────────────

// TestMigrationRunnerBaseline verifies that DBInit on a fresh database applies
// 0001_baseline.sql and records it in schema_migrations.
func TestMigrationRunnerBaseline(test *testing.T) {
	dbpath := "testmigrationrunner.sqlite"
	os.Remove(dbpath)
	db := DBInit(dbpath, MockLoganne{})

	if !db.TableExists("schema_migrations") {
		test.Fatal("schema_migrations table not created")
	}

	var exists int
	db.DB.Get(&exists, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = '0001_baseline.sql')")
	if exists != 1 {
		test.Error("0001_baseline.sql not recorded in schema_migrations")
	}

	for _, table := range []string{"track", "predicate", "tag", "collection", "collection_track", "album", "artist"} {
		if !db.TableExists(table) {
			test.Errorf("table %q missing after baseline migration", table)
		}
	}

	os.Remove(dbpath)
}

// TestMigrationRunnerIdempotent verifies that calling DBInit twice does not
// re-apply migrations or create duplicate records in schema_migrations.
func TestMigrationRunnerIdempotent(test *testing.T) {
	dbpath := "testmigrationrunneridempotent.sqlite"
	os.Remove(dbpath)
	DBInit(dbpath, MockLoganne{}) // first boot
	db := DBInit(dbpath, MockLoganne{}) // second boot

	var count int
	db.DB.Get(&count, "SELECT COUNT(*) FROM schema_migrations WHERE version = '0001_baseline.sql'")
	if count != 1 {
		test.Errorf("expected exactly 1 schema_migrations record for 0001_baseline.sql after two boots, got %d", count)
	}

	os.Remove(dbpath)
}

// TestMigrationRunnerOrdering verifies that multiple migrations are applied in
// lexical order and each recorded exactly once, with idempotent re-run.
func TestMigrationRunnerOrdering(test *testing.T) {
	dbpath := "testmigrationrunnerordering.sqlite"
	os.Remove(dbpath)
	db := sqlx.MustConnect("sqlite3", dbpath+"?_busy_timeout=10000")
	db.MustExec("PRAGMA journal_mode=WAL;")
	db.MustExec("PRAGMA foreign_keys = ON;")
	store := Datastore{DB: db, Loganne: MockLoganne{}, infoCache: new(atomic.Pointer[InfoMetricsSnapshot])}

	// Two migrations: create a table, then add a column — order matters.
	testFS := fstest.MapFS{
		"0001_create.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS migtest (id INTEGER PRIMARY KEY);`)},
		"0002_add_col.sql": &fstest.MapFile{Data: []byte(`ALTER TABLE migtest ADD COLUMN label TEXT;`)},
	}

	store.applyMigrationsFromFS(testFS, "*.sql")

	// Both migrations recorded
	var count int
	store.DB.Get(&count, "SELECT COUNT(*) FROM schema_migrations")
	assertEqual(test, "both migrations recorded", 2, count)

	// Table and column both created in the right order
	if !store.TableExists("migtest") {
		test.Error("migtest table not created by 0001_create.sql")
	}
	if !store.ColExists("migtest", "label") {
		test.Error("label column not added by 0002_add_col.sql")
	}

	// Re-running is idempotent — no duplicate records
	store.applyMigrationsFromFS(testFS, "*.sql")
	store.DB.Get(&count, "SELECT COUNT(*) FROM schema_migrations")
	assertEqual(test, "still 2 records after idempotent re-run", 2, count)

	os.Remove(dbpath)
}
