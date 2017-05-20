package main

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"log"
)

var db *sql.DB
var dbpath = "./media.sqlite"

func DBInit() {
	var err error
	db, err = sql.Open("sqlite3", dbpath)
	if err != nil {
		log.Fatal(err)
	}
	
	if (!DBTableExists("track")) {
		sqlStmt := `
		CREATE TABLE "track" ("id" INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL, "fingerprint" TEXT UNIQUE, "url" TEXT UNIQUE, "duration" INTEGER);
		`
		_, err := db.Exec(sqlStmt)
		if err != nil {
			log.Fatalf("%q: %s\n", err, sqlStmt)
		}
	}
}

func DBTableExists(tablename string) bool {
	var count int
	stmt, err := db.Prepare("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?")
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