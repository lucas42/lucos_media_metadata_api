package main

import (
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"log/slog"
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
			FOREIGN KEY (predicateid) REFERENCES predicate(id),
			CONSTRAINT track_predicate_unique UNIQUE (trackid, predicateid)
		);
		`
		database.DB.MustExec(sqlStmt)
	}
	if !database.TableExists("collection") {
		slog.Info("Creating table `collection`")
		sqlStmt := `
		CREATE TABLE "collection" (
			"slug" TEXT PRIMARY KEY NOT NULL,
			"name" TEXT UNIQUE NOT NULL
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
	return
}

func (store Datastore) TableExists(tablename string) (found bool) {
	err := store.DB.Get(&found, "SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ?", tablename)
	if err != nil && err.Error() != "sql: no rows in result set" {
		panic(err)
	}
	return
}
