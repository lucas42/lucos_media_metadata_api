package main

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)


// Test copySQLiteDB creates a temp copy
func TestCopySQLiteDB(t *testing.T) {
	// create a temporary DB file
	tmpDir := t.TempDir()
	dbFile := filepath.Join(tmpDir, "test.db")
	f, err := os.Create(dbFile)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	copyPath, cleanup, err := copySQLiteDB(dbFile)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	if _, err := os.Stat(copyPath); os.IsNotExist(err) {
		t.Errorf("expected copied DB file to exist at %s", copyPath)
	}
}