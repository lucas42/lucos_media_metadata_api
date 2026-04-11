package main

import (
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"log/slog"
	"strings"
)

/**
 * A struct for wrapping a database
 */
type Datastore struct {
	DB *sqlx.DB
	Loganne LoganneInterface
	ManagerOrigin string
}

func DBInit(dbpath string, loganne LoganneInterface) (database Datastore) {
	db := sqlx.MustConnect("sqlite3", dbpath+"?_busy_timeout=10000")
	database = Datastore{DB: db, Loganne: loganne}
	database.DB.MustExec("PRAGMA journal_mode=WAL;")
	database.DB.MustExec("PRAGMA foreign_keys = ON;")
	if !database.TableExists("track") {
		slog.Info("Creating table `track`")
		sqlStmt := `
		CREATE TABLE "track" (
			"id" INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL, 
			"fingerprint" TEXT UNIQUE, 
			"url" TEXT UNIQUE, 
			"duration" INTEGER,
			"weighting" FLOAT NOT NULL DEFAULT 0,
			"cum_weighting" FLOAT NOT NULL DEFAULT 0
		);
		`
		database.DB.MustExec(sqlStmt)
	}
	if !database.TableExists("predicate") {
		slog.Info("Creating table `predicate`")
		sqlStmt := `
		CREATE TABLE "predicate" (
			"id" TEXT PRIMARY KEY NOT NULL
		);
		`
		database.DB.MustExec(sqlStmt)
	}
	if !database.TableExists("tag") {
		slog.Info("Creating table `tag`")
		sqlStmt := `
		CREATE TABLE "tag" (
			"trackid" TEXT NOT NULL,
			"predicateid" TEXT NOT NULL,
			"value" TEXT,
			"uri" TEXT DEFAULT "",
			FOREIGN KEY (trackid) REFERENCES track(id),
			FOREIGN KEY (predicateid) REFERENCES predicate(id)
		);
		`
		database.DB.MustExec(sqlStmt)
	}
	if database.hasTagUniqueConstraint() {
		database.migrateTagTableDropUnique()
	}
	if !database.ColExists("tag", "uri") {
		slog.Info("Updating table `tag` to add uri column")
		database.DB.MustExec(`ALTER TABLE "tag" ADD COLUMN "uri" TEXT DEFAULT "";`)
	}
	if !database.TableExists("collection") {
		slog.Info("Creating table `collection`")
		sqlStmt := `
		CREATE TABLE "collection" (
			"slug" TEXT PRIMARY KEY NOT NULL,
			"name" TEXT UNIQUE NOT NULL,
			"icon" TEXT DEFAULT ""
		);
		`
		database.DB.MustExec(sqlStmt)
	}
	if !database.TableExists("collection_track") {
		slog.Info("Creating table `collection_track`")
		sqlStmt := `
		CREATE TABLE "collection_track" (
			"collectionslug" TEXT NOT NULL,
			"trackid" TEXT NOT NULL,
			"cum_weighting" FLOAT NOT NULL DEFAULT 0,
			FOREIGN KEY (collectionslug) REFERENCES collection(slug),
			FOREIGN KEY (trackid) REFERENCES track(id),
			CONSTRAINT track_collection_unique UNIQUE (collectionslug, trackid)
		);
		`
		database.DB.MustExec(sqlStmt)
	}
	if !database.ColExists("collection", "icon") {
		slog.Info("Updating table `collection` to add icon field")
		sqlStmt := `
		ALTER TABLE "collection" ADD COLUMN "icon" TEXT DEFAULT "";
		`
		database.DB.MustExec(sqlStmt)

		// The previous convention was to have the Icon as part of the name, separated by a space.
		// Migrate all the existing collection names to use separate fields for each
		collections, err := database.getAllCollections()
		if err != nil {
			panic(err)
		}
		for i := range collections {
			oldCollection := &collections[i]
			subName := strings.SplitN(oldCollection.Name, " ", 2)
			newCollection := Collection{
				Slug: oldCollection.Slug,
				Name: subName[1],
				Icon: subName[0],
			}
			database.updateCreateCollection(*oldCollection, newCollection, "all")
		}
	}

	if !database.TableExists("album") {
		slog.Info("Creating table `album`")
		sqlStmt := `
		CREATE TABLE "album" (
			"id" INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
			"name" TEXT NOT NULL UNIQUE
		);
		`
		database.DB.MustExec(sqlStmt)
	}

	// Migrate mentions/about/language tags to use uri column and import names from eolas
	if database.needsEolasDataMigration() {
		database.migrateEolasData()
	}
	return
}

func (store Datastore) TableExists(tablename string) (found bool) {
	err := store.DB.Get(&found, "SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ?", tablename)
	if err != nil && err.Error() != "sql: no rows in result set" {
		panic(err)
	}
	return
}
func (store Datastore) ColExists(tablename string, colname string) (found bool) {
	err := store.DB.Get(&found, "SELECT 1 FROM pragma_table_info(?) WHERE name= ?", tablename, colname)
	if err != nil && err.Error() != "sql: no rows in result set" {
		panic(err)
	}
	return
}

func (store Datastore) hasTagUniqueConstraint() bool {
	// Check the CREATE TABLE SQL for a UNIQUE keyword.
	// SQLite stores auto-indexes from CONSTRAINT...UNIQUE as sqlite_autoindex_tag_N,
	// not the constraint name, so we can't match by index name.
	var sql string
	err := store.DB.Get(&sql, "SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'tag'")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToUpper(sql), "UNIQUE")
}

func (store Datastore) migrateTagTableDropUnique() {
	slog.Info("Migrating `tag` table: dropping UNIQUE constraint and splitting multi-value CSV fields")

	// Count rows before migration for validation
	var beforeCount int
	store.DB.Get(&beforeCount, "SELECT COUNT(*) FROM tag")
	slog.Info("Migration audit", "tag_rows_before", beforeCount)

	// Foreign keys must be off for table rebuild
	store.DB.MustExec("PRAGMA foreign_keys = OFF;")
	tx := store.DB.MustBegin()

	tx.MustExec(`
		CREATE TABLE "tag_new" (
			"trackid" TEXT NOT NULL,
			"predicateid" TEXT NOT NULL,
			"value" TEXT,
			"uri" TEXT DEFAULT "",
			FOREIGN KEY (trackid) REFERENCES track(id),
			FOREIGN KEY (predicateid) REFERENCES predicate(id)
		);
	`)
	tx.MustExec(`INSERT INTO tag_new(trackid, predicateid, value) SELECT trackid, predicateid, value FROM tag;`)
	tx.MustExec(`DROP TABLE tag;`)
	tx.MustExec(`ALTER TABLE tag_new RENAME TO tag;`)

	// Split comma-separated values for multi-value predicates
	for predicate, config := range predicateRegistry {
		if !config.MultiValue {
			continue
		}
		var rows []struct {
			TrackID     string `db:"trackid"`
			PredicateID string `db:"predicateid"`
			Value       string `db:"value"`
		}
		err := tx.Select(&rows, "SELECT trackid, predicateid, value FROM tag WHERE predicateid = ?", predicate)
		if err != nil {
			panic(err)
		}
		for _, row := range rows {
			if !strings.Contains(row.Value, ",") {
				continue
			}
			// Delete the original CSV row
			tx.MustExec("DELETE FROM tag WHERE trackid = ? AND predicateid = ? AND value = ?", row.TrackID, row.PredicateID, row.Value)
			// Insert individual values
			parts := strings.Split(row.Value, ",")
			for _, part := range parts {
				trimmed := strings.TrimSpace(part)
				if trimmed == "" {
					continue
				}
				tx.MustExec("INSERT INTO tag(trackid, predicateid, value) VALUES(?, ?, ?)", row.TrackID, row.PredicateID, trimmed)
			}
			slog.Info("Split CSV value", "predicate", predicate, "trackid", row.TrackID, "original", row.Value, "parts", len(parts))
		}
	}

	err := tx.Commit()
	if err != nil {
		panic(err)
	}
	store.DB.MustExec("PRAGMA foreign_keys = ON;")

	// Count rows after migration for validation
	var afterCount int
	store.DB.Get(&afterCount, "SELECT COUNT(*) FROM tag")
	slog.Info("Migration audit", "tag_rows_after", afterCount)
	slog.Info("Migration complete: tag table UNIQUE constraint removed, CSV values split")
}

// needsEolasDataMigration checks whether the tag table contains unmigrated
// mentions/about/language data that should be enriched from eolas.
func (store Datastore) needsEolasDataMigration() bool {
	// mentions/about: value looks like a URI but uri column is empty
	var uriLikeCount int
	store.DB.Get(&uriLikeCount, `SELECT COUNT(*) FROM tag WHERE predicateid IN ('mentions', 'about') AND value LIKE 'http%' AND (uri = '' OR uri IS NULL)`)
	if uriLikeCount > 0 {
		return true
	}
	// language: value is a short code and uri column is empty
	var langCount int
	store.DB.Get(&langCount, `SELECT COUNT(*) FROM tag WHERE predicateid = 'language' AND (uri = '' OR uri IS NULL)`)
	return langCount > 0
}

// migrateEolasData migrates mentions/about/language tags to use the uri column
// and imports human-readable names from lucos_eolas.
func (store Datastore) migrateEolasData() {
	slog.Info("Starting eolas data migration for mentions/about/language tags")

	type tagRow struct {
		TrackID     int    `db:"trackid"`
		PredicateID string `db:"predicateid"`
		Value       string `db:"value"`
	}

	// Collect mentions/about tags where value is a URI
	var mentionsAboutTags []tagRow
	err := store.DB.Select(&mentionsAboutTags, `SELECT trackid, predicateid, value FROM tag WHERE predicateid IN ('mentions', 'about') AND value LIKE 'http%' AND (uri = '' OR uri IS NULL)`)
	if err != nil {
		slog.Warn("Failed to query mentions/about tags for migration", slog.Any("error", err))
		return
	}

	// Collect language tags
	var languageTags []tagRow
	err = store.DB.Select(&languageTags, `SELECT trackid, predicateid, value FROM tag WHERE predicateid = 'language' AND (uri = '' OR uri IS NULL)`)
	if err != nil {
		slog.Warn("Failed to query language tags for migration", slog.Any("error", err))
		return
	}

	if len(mentionsAboutTags) == 0 && len(languageTags) == 0 {
		slog.Info("No tags need eolas migration")
		return
	}

	slog.Info("Tags needing migration", "mentions_about", len(mentionsAboutTags), "language", len(languageTags))

	// Collect all URIs we need to look up in eolas
	uriSet := make(map[string]bool)
	for _, tag := range mentionsAboutTags {
		uriSet[tag.Value] = true
	}
	for _, tag := range languageTags {
		uri := eolasLanguageURI(tag.Value)
		uriSet[uri] = true
	}
	uris := make([]string, 0, len(uriSet))
	for uri := range uriSet {
		uris = append(uris, uri)
	}

	// Fetch names from eolas
	names := fetchEolasNames(uris)
	if names == nil {
		names = make(map[string]string)
	}
	slog.Info("Eolas name lookup complete", "requested", len(uris), "resolved", len(names))

	// Migrate in a single transaction
	tx, err := store.DB.Beginx()
	if err != nil {
		slog.Warn("Failed to begin migration transaction", slog.Any("error", err))
		return
	}

	migratedCount := 0

	// Migrate mentions/about: move URI from value to uri, set name as value
	for _, tag := range mentionsAboutTags {
		name := names[tag.Value]
		if name == "" {
			// If eolas didn't return a name, keep the URI as fallback name
			name = tag.Value
		}
		_, err = tx.Exec(`UPDATE tag SET uri = ?, value = ? WHERE trackid = ? AND predicateid = ? AND value = ? AND (uri = '' OR uri IS NULL)`,
			tag.Value, name, tag.TrackID, tag.PredicateID, tag.Value)
		if err != nil {
			slog.Warn("Failed to migrate mentions/about tag", slog.Any("error", err), "trackid", tag.TrackID, "predicate", tag.PredicateID)
			_ = tx.Rollback()
			return
		}
		migratedCount++
	}

	// Migrate language: build URI from code, set name as value
	for _, tag := range languageTags {
		uri := eolasLanguageURI(tag.Value)
		name := names[uri]
		if name == "" {
			// If eolas didn't return a name, keep the code as the name
			name = tag.Value
		}
		_, err = tx.Exec(`UPDATE tag SET uri = ?, value = ? WHERE trackid = ? AND predicateid = ? AND value = ? AND (uri = '' OR uri IS NULL)`,
			uri, name, tag.TrackID, tag.PredicateID, tag.Value)
		if err != nil {
			slog.Warn("Failed to migrate language tag", slog.Any("error", err), "trackid", tag.TrackID, "predicate", tag.PredicateID)
			_ = tx.Rollback()
			return
		}
		migratedCount++
	}

	err = tx.Commit()
	if err != nil {
		slog.Warn("Failed to commit eolas migration", slog.Any("error", err))
		return
	}

	slog.Info("Eolas data migration complete", "migrated_tags", migratedCount)
}
