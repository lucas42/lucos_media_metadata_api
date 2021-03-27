package main

import (
	"os"
	"testing"
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
	if !db.TableExists("global") {
		test.Error("global table not created")
	}
	if db.TableExists("moo-moo head") {
		test.Error("Unexpected table created")
	}
	os.Remove(dbpath)
}
