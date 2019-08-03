package main

import (
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

/**
 * A struct for wrapping a database
 */
type Datastore struct {
	DB *sqlx.DB
}

func DBInit(dbpath string) (database Datastore) {
	db := sqlx.MustConnect("sqlite3", dbpath)
	database = Datastore{db}
	database.DB.MustExec("PRAGMA foreign_keys = ON;")
	if !database.TableExists("track") {
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
	if !database.TableExists("global") {
		sqlStmt := `
		CREATE TABLE "global" (
			"key" TEXT PRIMARY KEY NOT NULL, 
			"value" TEXT
		);
		`
		database.DB.MustExec(sqlStmt)
	}
	if !database.TableExists("predicate") {
		sqlStmt := `
		CREATE TABLE "predicate" (
			"id" TEXT PRIMARY KEY NOT NULL
		);
		`
		database.DB.MustExec(sqlStmt)
	}
	if !database.TableExists("tag") {
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
	return
}

func (store Datastore) TableExists(tablename string) (found bool) {
	err := store.DB.Get(&found, "SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ?", tablename)
	if err != nil && err.Error() != "sql: no rows in result set" {
		panic(err)
	}
	return
}
