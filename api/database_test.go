package main

import (
	"encoding/json"
	"os"
	"testing"
	"github.com/jmoiron/sqlx"
)

func TestDatabaseSetup(test *testing.T) {
	dbpath := "testdb.sqlite"
	os.Remove(dbpath)
	db := DBInit(dbpath, MockLoganne{})
	if !db.TableExists("track") {
		test.Error("track table not created")
	}
	if !db.TableExists("predicate") {
		test.Error("predicate table not created")
	}
	if !db.TableExists("tag") {
		test.Error("tag table not created")
	}
	if !db.TableExists("collection") {
		test.Error("collection table not created")
	}
	if !db.TableExists("collection_track") {
		test.Error("collection_track table not created")
	}
	if db.TableExists("global") {
		test.Error("Unused `global` table created")
	}
	if db.TableExists("moo-moo head") {
		test.Error("Unexpected table created")
	}
	os.Remove(dbpath)
}

func TestUpgradeCollectionTable(test *testing.T) {
	dbpath := "testdb.sqlite"
	os.Remove(dbpath)
	db := sqlx.MustConnect("sqlite3", dbpath+"?_busy_timeout=10000")

	// Create historical version of collection table
	sqlStmt := `
	CREATE TABLE "collection" (
		"slug" TEXT PRIMARY KEY NOT NULL,
		"name" TEXT UNIQUE NOT NULL
	);
	INSERT INTO collection(slug, name) values("christmas", "🎄 Christmas");
	INSERT INTO collection(slug, name) values("bath", "🫧 Bathtime");
	INSERT INTO collection(slug, name) values("trans", "🏳️‍⚧️ Tranz Tunez");
	`
	db.MustExec(sqlStmt)
	datastore := DBInit(dbpath, MockLoganne{})
	if !datastore.TableExists("collection") {
		test.Error("collection table missing")
	}
	collections, err := datastore.getAllCollections()
	if err != nil {
		test.Error(err)
		return
	}
	assertEqual(test, "Upgraded collection table slug", "christmas", collections[0].Slug)
	assertEqual(test, "Upgraded collection table name", "Christmas", collections[0].Name)
	assertEqual(test, "Upgraded collection table icon", "🎄", collections[0].Icon)
	assertEqual(test, "Upgraded collection table slug", "bath", collections[1].Slug)
	assertEqual(test, "Upgraded collection table name", "Bathtime", collections[1].Name)
	assertEqual(test, "Upgraded collection table icon", "🫧", collections[1].Icon)
	assertEqual(test, "Upgraded collection table slug", "trans", collections[2].Slug)
	assertEqual(test, "Upgraded collection table name", "Tranz Tunez", collections[2].Name)
	assertEqual(test, "Upgraded collection table icon", "🏳️‍⚧️", collections[2].Icon)
	os.Remove(dbpath)
}

func TestMigrateTagTableDropUnique(test *testing.T) {
	dbpath := "testmigration.sqlite"
	os.Remove(dbpath)
	db := sqlx.MustConnect("sqlite3", dbpath+"?_busy_timeout=10000")

	// Create old-style tables with UNIQUE constraint
	db.MustExec(`
		CREATE TABLE "track" (
			"id" INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
			"fingerprint" TEXT UNIQUE,
			"url" TEXT UNIQUE,
			"duration" INTEGER,
			"weighting" FLOAT NOT NULL DEFAULT 0,
			"cum_weighting" FLOAT NOT NULL DEFAULT 0
		);
		CREATE TABLE "predicate" ("id" TEXT PRIMARY KEY NOT NULL);
		CREATE TABLE "tag" (
			"trackid" TEXT NOT NULL,
			"predicateid" TEXT NOT NULL,
			"value" TEXT,
			FOREIGN KEY (trackid) REFERENCES track(id),
			FOREIGN KEY (predicateid) REFERENCES predicate(id),
			CONSTRAINT track_predicate_unique UNIQUE (trackid, predicateid)
		);
	`)

	// Insert test data: a track with CSV values in multi-value predicates
	db.MustExec(`INSERT INTO track(id, url, fingerprint, duration) VALUES(1, 'http://example.com/track1', 'fp1', 180)`)
	db.MustExec(`INSERT INTO predicate(id) VALUES('composer'), ('producer'), ('title'), ('language')`)
	db.MustExec(`INSERT INTO tag(trackid, predicateid, value) VALUES(1, 'composer', 'Bach,Mozart')`)
	db.MustExec(`INSERT INTO tag(trackid, predicateid, value) VALUES(1, 'title', 'Symphony No. 5')`)
	db.MustExec(`INSERT INTO tag(trackid, predicateid, value) VALUES(1, 'language', 'en, fr')`)
	db.MustExec(`INSERT INTO tag(trackid, predicateid, value) VALUES(1, 'producer', 'SingleProducer')`)

	db.Close()

	// Run DBInit which should trigger migration
	datastore := DBInit(dbpath, MockLoganne{})

	// Verify UNIQUE constraint is gone
	if datastore.hasTagUniqueConstraint() {
		test.Error("UNIQUE constraint still exists after migration")
	}

	// Verify CSV values were split for multi-value predicates
	var composerTags []Tag
	datastore.DB.Select(&composerTags, "SELECT trackid, predicateid, value FROM tag WHERE predicateid = 'composer' AND trackid = 1 ORDER BY value")
	assertEqual(test, "composer tag count", 2, len(composerTags))
	if len(composerTags) == 2 {
		assertEqual(test, "first composer", "Bach", composerTags[0].Value)
		assertEqual(test, "second composer", "Mozart", composerTags[1].Value)
	}

	// Verify language was split with whitespace trimming
	var langTags []Tag
	datastore.DB.Select(&langTags, "SELECT trackid, predicateid, value FROM tag WHERE predicateid = 'language' AND trackid = 1 ORDER BY value")
	assertEqual(test, "language tag count", 2, len(langTags))
	if len(langTags) == 2 {
		assertEqual(test, "first language", "en", langTags[0].Value)
		assertEqual(test, "second language", "fr", langTags[1].Value)
	}

	// Verify single-value predicates were NOT split
	var titleTags []Tag
	datastore.DB.Select(&titleTags, "SELECT trackid, predicateid, value FROM tag WHERE predicateid = 'title' AND trackid = 1")
	assertEqual(test, "title tag count", 1, len(titleTags))
	if len(titleTags) == 1 {
		assertEqual(test, "title value unchanged", "Symphony No. 5", titleTags[0].Value)
	}

	// Verify non-CSV multi-value predicates were left alone
	var producerTags []Tag
	datastore.DB.Select(&producerTags, "SELECT trackid, predicateid, value FROM tag WHERE predicateid = 'producer' AND trackid = 1")
	assertEqual(test, "producer tag count", 1, len(producerTags))
	if len(producerTags) == 1 {
		assertEqual(test, "producer value unchanged", "SingleProducer", producerTags[0].Value)
	}

	// Verify that duplicate (trackid, predicateid) rows are now allowed
	_, err := datastore.DB.Exec("INSERT INTO tag(trackid, predicateid, value) VALUES(1, 'composer', 'Beethoven')")
	if err != nil {
		test.Errorf("Should allow duplicate (trackid, predicateid): %v", err)
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

func TestTagListMarshalJSONMultiValue(test *testing.T) {
	tags := TagList{
		{PredicateID: "title", Value: "My Song"},
		{PredicateID: "composer", Value: "Bach"},
		{PredicateID: "composer", Value: "Mozart"},
		{PredicateID: "language", Value: "en"},
		{PredicateID: "language", Value: "fr"},
	}
	data, err := json.Marshal(tags)
	if err != nil {
		test.Fatalf("MarshalJSON failed: %v", err)
	}
	var m map[string]string
	json.Unmarshal(data, &m)
	assertEqual(test, "title should be single value", "My Song", m["title"])
	assertEqual(test, "composer should be comma-joined", "Bach,Mozart", m["composer"])
	assertEqual(test, "language should be comma-joined", "en,fr", m["language"])
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

func TestMigrateMultiValueCSVSplitFixesReCorruptedData(test *testing.T) {
	dbpath := "testmigratecsv.sqlite"
	os.Remove(dbpath)
	datastore := DBInit(dbpath, MockLoganne{})

	// Simulate re-corrupted data: CSV value written after original migration
	datastore.DB.MustExec(`INSERT INTO track(id, url, fingerprint, duration) VALUES(1, 'http://example.com/t1', 'fp1', 100)`)
	datastore.DB.MustExec(`INSERT INTO predicate(id) VALUES('language')`)
	datastore.DB.MustExec(`INSERT INTO predicate(id) VALUES('title')`)
	datastore.DB.MustExec(`INSERT INTO tag(trackid, predicateid, value) VALUES(1, 'language', 'en,fr')`)
	datastore.DB.MustExec(`INSERT INTO tag(trackid, predicateid, value) VALUES(1, 'title', 'Hello, World')`)

	// Re-run init to trigger migration
	datastore2 := DBInit(dbpath, MockLoganne{})

	// Multi-value predicate CSV should be split
	var langValues []string
	datastore2.DB.Select(&langValues, "SELECT value FROM tag WHERE trackid = 1 AND predicateid = 'language' ORDER BY rowid")
	if len(langValues) != 2 {
		test.Fatalf("Expected 2 language values, got %d: %v", len(langValues), langValues)
	}
	assertEqual(test, "first language", "en", langValues[0])
	assertEqual(test, "second language", "fr", langValues[1])

	// Single-value predicate should be left alone
	var titleValue string
	datastore2.DB.Get(&titleValue, "SELECT value FROM tag WHERE trackid = 1 AND predicateid = 'title'")
	assertEqual(test, "title should be preserved", "Hello, World", titleValue)

	os.Remove(dbpath)
}
