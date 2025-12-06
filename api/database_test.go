package main

import (
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