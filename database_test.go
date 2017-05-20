package main

import (
    "os"
    "testing"
)

func TestDatabaseSetup(test *testing.T) {
	os.Remove(dbpath)
	DBInit()
	if (!DBTableExists("track")) {
		test.Error("track table not created")
	}
	//os.Remove(dbpath)
}