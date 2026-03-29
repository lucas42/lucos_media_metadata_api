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
}

func DBInit(dbpath string, loganne LoganneInterface) (database Datastore) {
	db := sqlx.MustConnect("sqlite3", dbpath+"?_busy_timeout=10000")
	database = Datastore{db, loganne}
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
			FOREIGN KEY (trackid) REFERENCES track(id),
			FOREIGN KEY (predicateid) REFERENCES predicate(id)
		);
		`
		database.DB.MustExec(sqlStmt)
	}
	if database.hasTagUniqueConstraint() {
		database.migrateTagTableDropUnique()
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
			FOREIGN KEY (trackid) REFERENCES track(id),
			FOREIGN KEY (predicateid) REFERENCES predicate(id)
		);
	`)
	tx.MustExec(`INSERT INTO tag_new SELECT * FROM tag;`)
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
