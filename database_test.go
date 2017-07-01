package main

import (
    "os"
    "testing"
)

func TestDatabaseSetup(test *testing.T) {
	dbpath := "testdb.sqlite"
	os.Remove(dbpath)
	db, err := DBInit(dbpath)
	if (err != nil) {
		test.Error(err)
	}
	if (!db.TableExists("track")) {
		test.Error("track table not created")
	}
	if (!db.TableExists("predicate")) {
		test.Error("predicate table not created")
	}
	if (!db.TableExists("tag")) {
		test.Error("tag table not created")
	}
	if (!db.TableExists("global")) {
		test.Error("global table not created")
	}
	os.Remove(dbpath)
}