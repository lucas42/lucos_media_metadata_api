package main

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"log"
)

/**
 * A struct for wrapping a database
 */
type Datastore struct {
	DB *sql.DB
}


func DBInit(dbpath string) (database Datastore, err error) {
	db, err := sql.Open("sqlite3", dbpath)
	if err != nil {
		return
	}
	database = Datastore{db}
	_, err = database.DB.Exec("PRAGMA foreign_keys = ON;")
	if err != nil { return }
	if (!database.TableExists("track")) {
		sqlStmt := `
		CREATE TABLE "track" ("id" INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL, "fingerprint" TEXT UNIQUE, "url" TEXT UNIQUE, "duration" INTEGER);
		`
		_, err = database.DB.Exec(sqlStmt)
		if err != nil { return }
	}

	if (!database.TableExists("global")) {
		sqlStmt := `
		CREATE TABLE "global" ("key" TEXT PRIMARY KEY NOT NULL, "value" TEXT);
		`
		_, err = database.DB.Exec(sqlStmt)
		if err != nil { return }
	}
	if (!database.TableExists("predicate")) {
		sqlStmt := `
		CREATE TABLE "predicate" ("id" TEXT PRIMARY KEY NOT NULL);
		`
		_, err = database.DB.Exec(sqlStmt)
		if err != nil { return }
	}
	if (!database.TableExists("tag")) {
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
		_, err = database.DB.Exec(sqlStmt)
		if err != nil { return }
	}
	return
}

func (store Datastore) TableExists(tablename string) bool {
	var count int
	stmt, err := store.DB.Prepare("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?")
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()
	err = stmt.QueryRow(tablename).Scan(&count)
	if err != nil {
		log.Fatal(err)
	}
	return (count > 0)
}