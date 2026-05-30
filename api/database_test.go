package main

import (
	"os"
	"strings"
	"testing"
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

func TestMigrateEolasDataMentionsAbout(test *testing.T) {
	dbpath := "testmigration_eolas_mentions.sqlite"
	os.Remove(dbpath)
	db := sqlx.MustConnect("sqlite3", dbpath+"?_busy_timeout=10000")

	// Create tables manually (without migration triggers)
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
			"uri" TEXT DEFAULT "",
			FOREIGN KEY (trackid) REFERENCES track(id),
			FOREIGN KEY (predicateid) REFERENCES predicate(id)
		);
		CREATE TABLE "collection" (
			"slug" TEXT PRIMARY KEY NOT NULL,
			"name" TEXT UNIQUE NOT NULL,
			"icon" TEXT DEFAULT ""
		);
		CREATE TABLE "collection_track" (
			"collectionslug" TEXT NOT NULL,
			"trackid" TEXT NOT NULL,
			"cum_weighting" FLOAT NOT NULL DEFAULT 0,
			FOREIGN KEY (collectionslug) REFERENCES collection(slug),
			FOREIGN KEY (trackid) REFERENCES track(id),
			CONSTRAINT track_collection_unique UNIQUE (collectionslug, trackid)
		);
	`)

	// Insert test data: mentions/about with URIs as values
	db.MustExec(`INSERT INTO track(id, url, fingerprint, duration) VALUES(1, 'http://example.com/t1', 'fp1', 100)`)
	db.MustExec(`INSERT INTO predicate(id) VALUES('mentions'), ('about'), ('title')`)
	db.MustExec(`INSERT INTO tag(trackid, predicateid, value) VALUES(1, 'mentions', 'https://eolas.l42.eu/metadata/person/alice/')`)
	db.MustExec(`INSERT INTO tag(trackid, predicateid, value) VALUES(1, 'about', 'https://eolas.l42.eu/metadata/topic/music/')`)
	db.MustExec(`INSERT INTO tag(trackid, predicateid, value) VALUES(1, 'title', 'My Song')`)
	db.Close()

	// DBInit should detect and run migration (eolas won't be available in tests, so fallback names apply)
	datastore := DBInit(dbpath, MockLoganne{})

	// Verify mentions tag was migrated: value should be the URI as fallback, uri should be set
	var mentionsTag Tag
	err := datastore.DB.Get(&mentionsTag, "SELECT trackid, predicateid, value, uri FROM tag WHERE predicateid = 'mentions'")
	if err != nil {
		test.Fatalf("Failed to query mentions tag: %v", err)
	}
	assertEqual(test, "mentions uri", "https://eolas.l42.eu/metadata/person/alice/", mentionsTag.URI)
	// Without eolas, the value falls back to the URI
	assertEqual(test, "mentions value (fallback)", "https://eolas.l42.eu/metadata/person/alice/", mentionsTag.Value)

	// Verify about tag was migrated
	var aboutTag Tag
	err = datastore.DB.Get(&aboutTag, "SELECT trackid, predicateid, value, uri FROM tag WHERE predicateid = 'about'")
	if err != nil {
		test.Fatalf("Failed to query about tag: %v", err)
	}
	assertEqual(test, "about uri", "https://eolas.l42.eu/metadata/topic/music/", aboutTag.URI)

	// Verify title tag was NOT affected
	var titleTag Tag
	err = datastore.DB.Get(&titleTag, "SELECT trackid, predicateid, value, uri FROM tag WHERE predicateid = 'title'")
	if err != nil {
		test.Fatalf("Failed to query title tag: %v", err)
	}
	assertEqual(test, "title value unchanged", "My Song", titleTag.Value)
	assertEqual(test, "title uri unchanged", "", titleTag.URI)

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

func TestMigrateEolasDataLanguage(test *testing.T) {
	dbpath := "testmigration_eolas_language.sqlite"
	os.Remove(dbpath)
	db := sqlx.MustConnect("sqlite3", dbpath+"?_busy_timeout=10000")

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
			"uri" TEXT DEFAULT "",
			FOREIGN KEY (trackid) REFERENCES track(id),
			FOREIGN KEY (predicateid) REFERENCES predicate(id)
		);
		CREATE TABLE "collection" (
			"slug" TEXT PRIMARY KEY NOT NULL,
			"name" TEXT UNIQUE NOT NULL,
			"icon" TEXT DEFAULT ""
		);
		CREATE TABLE "collection_track" (
			"collectionslug" TEXT NOT NULL,
			"trackid" TEXT NOT NULL,
			"cum_weighting" FLOAT NOT NULL DEFAULT 0,
			FOREIGN KEY (collectionslug) REFERENCES collection(slug),
			FOREIGN KEY (trackid) REFERENCES track(id),
			CONSTRAINT track_collection_unique UNIQUE (collectionslug, trackid)
		);
	`)

	db.MustExec(`INSERT INTO track(id, url, fingerprint, duration) VALUES(1, 'http://example.com/t1', 'fp1', 100)`)
	db.MustExec(`INSERT INTO predicate(id) VALUES('language')`)
	db.MustExec(`INSERT INTO tag(trackid, predicateid, value) VALUES(1, 'language', 'en')`)
	db.MustExec(`INSERT INTO tag(trackid, predicateid, value) VALUES(1, 'language', 'fr')`)
	db.Close()

	datastore := DBInit(dbpath, MockLoganne{})

	// Verify language tags were migrated
	var langTags []Tag
	datastore.DB.Select(&langTags, "SELECT trackid, predicateid, value, uri FROM tag WHERE predicateid = 'language' ORDER BY uri")
	assertEqual(test, "language tag count", 2, len(langTags))

	if len(langTags) == 2 {
		// Without eolas, value falls back to the code
		assertEqual(test, "en uri", "https://eolas.l42.eu/metadata/language/en/", langTags[0].URI)
		assertEqual(test, "en value (fallback)", "en", langTags[0].Value)
		assertEqual(test, "fr uri", "https://eolas.l42.eu/metadata/language/fr/", langTags[1].URI)
		assertEqual(test, "fr value (fallback)", "fr", langTags[1].Value)
	}

	os.Remove(dbpath)
}

// TestMigrateArtistTags verifies that the artist tag migration backfills the uri
// column for old-style artist tags (value=name, uri=NULL) by creating Artist
// entities and populating the URI.
func TestMigrateArtistTags(test *testing.T) {
	dbpath := "testmigration_artist_tags.sqlite"
	os.Remove(dbpath)
	db := sqlx.MustConnect("sqlite3", dbpath+"?_busy_timeout=10000")

	// Pre-create all required tables to simulate a pre-migration database.
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
			"uri" TEXT DEFAULT "",
			FOREIGN KEY (trackid) REFERENCES track(id),
			FOREIGN KEY (predicateid) REFERENCES predicate(id)
		);
		CREATE TABLE "collection" (
			"slug" TEXT PRIMARY KEY NOT NULL,
			"name" TEXT UNIQUE NOT NULL,
			"icon" TEXT DEFAULT ""
		);
		CREATE TABLE "collection_track" (
			"collectionslug" TEXT NOT NULL,
			"trackid" TEXT NOT NULL,
			FOREIGN KEY (collectionslug) REFERENCES collection(slug),
			FOREIGN KEY (trackid) REFERENCES track(id),
			CONSTRAINT track_collection_unique UNIQUE (collectionslug, trackid)
		);
		CREATE TABLE "album" (
			"id" INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
			"name" TEXT NOT NULL UNIQUE
		);
	`)

	// Old-style artist tags: value=name, uri=empty (pre-SearchURL-removal state)
	db.MustExec(`INSERT INTO track(id, url, fingerprint, duration) VALUES(1, 'http://example.com/t1', 'fp1', 100)`)
	db.MustExec(`INSERT INTO track(id, url, fingerprint, duration) VALUES(2, 'http://example.com/t2', 'fp2', 120)`)
	db.MustExec(`INSERT INTO predicate(id) VALUES('artist'), ('title')`)
	db.MustExec(`INSERT INTO tag(trackid, predicateid, value, uri) VALUES(1, 'artist', 'Enya', '')`)
	db.MustExec(`INSERT INTO tag(trackid, predicateid, value, uri) VALUES(2, 'artist', 'Enya', '')`)  // same artist, two tracks
	db.MustExec(`INSERT INTO tag(trackid, predicateid, value, uri) VALUES(1, 'artist', 'Clannad', '')`) // second artist on track 1
	db.MustExec(`INSERT INTO tag(trackid, predicateid, value, uri) VALUES(1, 'title', 'Orinoco Flow', '')`) // non-artist tag untouched
	db.Close()

	// DBInit should detect unmigrated artist tags and run the backfill.
	// MEDIA_METADATA_MANAGER_ORIGIN is unset in tests, so URIs are like "/artists/{id}".
	datastore := DBInit(dbpath, MockLoganne{})

	// Verify all artist tags now have a uri set.
	var artistTags []Tag
	datastore.DB.Select(&artistTags, "SELECT trackid, predicateid, value, uri FROM tag WHERE predicateid = 'artist' ORDER BY trackid, value")
	assertEqual(test, "artist tag count", 3, len(artistTags))
	for _, tag := range artistTags {
		if tag.URI == "" {
			test.Errorf("artist tag for %q on track %d still has empty uri after migration", tag.Value, tag.TrackID)
		}
		if !strings.Contains(tag.URI, "/artists/") {
			test.Errorf("artist tag uri %q doesn't look like an artist URI", tag.URI)
		}
	}

	// Verify only one Artist entity was created for "Enya" (two tracks, one entity).
	var enyaCount int
	datastore.DB.Get(&enyaCount, "SELECT COUNT(*) FROM artist WHERE name = 'Enya'")
	assertEqual(test, "only one Enya artist entity", 1, enyaCount)

	// Verify "Clannad" got its own entity.
	var clannadCount int
	datastore.DB.Get(&clannadCount, "SELECT COUNT(*) FROM artist WHERE name = 'Clannad'")
	assertEqual(test, "only one Clannad artist entity", 1, clannadCount)

	// Verify both "Enya" tag rows point to the same URI.
	var enyaTags []Tag
	datastore.DB.Select(&enyaTags, "SELECT uri FROM tag WHERE predicateid = 'artist' AND value = 'Enya'")
	if len(enyaTags) == 2 && enyaTags[0].URI != enyaTags[1].URI {
		test.Errorf("both Enya artist tags should point to the same URI, got %q and %q", enyaTags[0].URI, enyaTags[1].URI)
	}

	// Verify non-artist tag was NOT touched.
	var titleTag Tag
	datastore.DB.Get(&titleTag, "SELECT value, uri FROM tag WHERE predicateid = 'title'")
	assertEqual(test, "title value unchanged", "Orinoco Flow", titleTag.Value)
	assertEqual(test, "title uri unchanged", "", titleTag.URI)

	os.Remove(dbpath)
	os.Remove(dbpath + "-shm")
	os.Remove(dbpath + "-wal")
}

// TestMigrateArtistTagsIdempotent verifies that the migration does not re-process
// artist tags that already have a uri populated.
func TestMigrateArtistTagsIdempotent(test *testing.T) {
	dbpath := "testmigration_artist_idempotent.sqlite"
	os.Remove(dbpath)
	db := sqlx.MustConnect("sqlite3", dbpath+"?_busy_timeout=10000")

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
			"uri" TEXT DEFAULT "",
			FOREIGN KEY (trackid) REFERENCES track(id),
			FOREIGN KEY (predicateid) REFERENCES predicate(id)
		);
		CREATE TABLE "collection" (
			"slug" TEXT PRIMARY KEY NOT NULL,
			"name" TEXT UNIQUE NOT NULL,
			"icon" TEXT DEFAULT ""
		);
		CREATE TABLE "collection_track" (
			"collectionslug" TEXT NOT NULL,
			"trackid" TEXT NOT NULL,
			FOREIGN KEY (collectionslug) REFERENCES collection(slug),
			FOREIGN KEY (trackid) REFERENCES track(id),
			CONSTRAINT track_collection_unique UNIQUE (collectionslug, trackid)
		);
		CREATE TABLE "album" (
			"id" INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
			"name" TEXT NOT NULL UNIQUE
		);
		CREATE TABLE "artist" (
			"id" INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
			"name" TEXT NOT NULL UNIQUE
		);
	`)

	// Already-migrated artist tag: uri is already set.
	db.MustExec(`INSERT INTO track(id, url, fingerprint, duration) VALUES(1, 'http://example.com/t1', 'fp1', 100)`)
	db.MustExec(`INSERT INTO predicate(id) VALUES('artist')`)
	db.MustExec(`INSERT INTO artist(id, name) VALUES(1, 'Enya')`)
	db.MustExec(`INSERT INTO tag(trackid, predicateid, value, uri) VALUES(1, 'artist', 'Enya', '/artists/1')`)
	db.Close()

	datastore := DBInit(dbpath, MockLoganne{})

	// Verify the existing tag was NOT modified.
	var tag Tag
	datastore.DB.Get(&tag, "SELECT value, uri FROM tag WHERE predicateid = 'artist'")
	assertEqual(test, "artist value unchanged", "Enya", tag.Value)
	assertEqual(test, "artist uri unchanged", "/artists/1", tag.URI)

	// Verify no duplicate artist entity was created.
	var count int
	datastore.DB.Get(&count, "SELECT COUNT(*) FROM artist")
	assertEqual(test, "still only one artist entity", 1, count)

	os.Remove(dbpath)
	os.Remove(dbpath + "-shm")
	os.Remove(dbpath + "-wal")
}

func TestMigrateEolasDataIdempotent(test *testing.T) {
	dbpath := "testmigration_eolas_idempotent.sqlite"
	os.Remove(dbpath)
	db := sqlx.MustConnect("sqlite3", dbpath+"?_busy_timeout=10000")

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
			"uri" TEXT DEFAULT "",
			FOREIGN KEY (trackid) REFERENCES track(id),
			FOREIGN KEY (predicateid) REFERENCES predicate(id)
		);
		CREATE TABLE "collection" (
			"slug" TEXT PRIMARY KEY NOT NULL,
			"name" TEXT UNIQUE NOT NULL,
			"icon" TEXT DEFAULT ""
		);
		CREATE TABLE "collection_track" (
			"collectionslug" TEXT NOT NULL,
			"trackid" TEXT NOT NULL,
			"cum_weighting" FLOAT NOT NULL DEFAULT 0,
			FOREIGN KEY (collectionslug) REFERENCES collection(slug),
			FOREIGN KEY (trackid) REFERENCES track(id),
			CONSTRAINT track_collection_unique UNIQUE (collectionslug, trackid)
		);
	`)

	// Insert already-migrated data (uri is already set)
	db.MustExec(`INSERT INTO track(id, url, fingerprint, duration) VALUES(1, 'http://example.com/t1', 'fp1', 100)`)
	db.MustExec(`INSERT INTO predicate(id) VALUES('mentions'), ('language')`)
	db.MustExec(`INSERT INTO tag(trackid, predicateid, value, uri) VALUES(1, 'mentions', 'Alice', 'https://eolas.l42.eu/metadata/person/alice/')`)
	db.MustExec(`INSERT INTO tag(trackid, predicateid, value, uri) VALUES(1, 'language', 'English', 'https://eolas.l42.eu/metadata/language/en/')`)
	db.Close()

	datastore := DBInit(dbpath, MockLoganne{})

	// Verify data was NOT changed (already migrated)
	var mentionsTag Tag
	datastore.DB.Get(&mentionsTag, "SELECT trackid, predicateid, value, uri FROM tag WHERE predicateid = 'mentions'")
	assertEqual(test, "mentions value unchanged", "Alice", mentionsTag.Value)
	assertEqual(test, "mentions uri unchanged", "https://eolas.l42.eu/metadata/person/alice/", mentionsTag.URI)

	var langTag Tag
	datastore.DB.Get(&langTag, "SELECT trackid, predicateid, value, uri FROM tag WHERE predicateid = 'language'")
	assertEqual(test, "language value unchanged", "English", langTag.Value)
	assertEqual(test, "language uri unchanged", "https://eolas.l42.eu/metadata/language/en/", langTag.URI)

	os.Remove(dbpath)
}
