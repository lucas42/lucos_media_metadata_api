package rdfgen

import (
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// Test splitting CSVs
func TestSplitCSV(t *testing.T) {
	input := "a, b ,c"
	expected := []string{"a", "b", "c"}
	got := splitCSV(input)
	for i := range expected {
		if expected[i] != got[i] {
			t.Errorf("expected %q, got %q", expected[i], got[i])
		}
	}
}

// Test mapPredicate returns URIs and terms
func TestMapPredicate(t *testing.T) {
	pred, terms := mapPredicate("title", "Song A")
	if pred != "http://www.w3.org/2004/02/skos/core#prefLabel" {
		t.Errorf("unexpected predicate URI: %s", pred)
	}
	if len(terms) != 1 {
		t.Errorf("expected 1 term, got %d", len(terms))
	}
}

// Test mapPredicate CSV handling
func TestMapPredicateCSV(t *testing.T) {
	pred, terms := mapPredicate("composer", "Alice,Bob")
	if len(terms) != 2 {
		t.Errorf("expected 2 terms, got %d", len(terms))
	}
	if pred != "http://purl.org/ontology/mo/composer" {
		t.Errorf("unexpected predicate: %s", pred)
	}
}

// Test multiple CSV fields per track
func TestMapPredicateMultipleCSV(t *testing.T) {
	pred1, terms1 := mapPredicate("composer", "Alice,Bob")
	pred2, terms2 := mapPredicate("producer", "Charlie, Dave ")
	if len(terms1) != 2 {
		t.Errorf("expected 2 composer terms, got %d", len(terms1))
	}
	if len(terms2) != 2 {
		t.Errorf("expected 2 producer terms, got %d", len(terms2))
	}
	for _, term := range append(terms1, terms2...) {
		if term.String() == "" {
			t.Errorf("expected non-empty RDF term")
		}
	}
	if pred1 != "http://purl.org/ontology/mo/composer" || pred2 != "http://purl.org/ontology/mo/producer" {
		t.Errorf("expected predicate URI to be contributor for CSV fields")
	}
}


// Test exportRDF with temporary DB
func TestExportRDF(t *testing.T) {
	// create temporary DB file
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Create tables
	_, err = db.Exec(`
	CREATE TABLE track (id INTEGER PRIMARY KEY, url TEXT, duration INTEGER);
	CREATE TABLE tag (trackid INTEGER, predicateid TEXT, value TEXT);
	`)
	if err != nil {
		t.Fatal(err)
	}

	// Insert a track and tags
	_, err = db.Exec(`INSERT INTO track (id, url, duration) VALUES (1, 'http://example.com', 120)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
	INSERT INTO tag (trackid, predicateid, value) VALUES
	(1, 'title', 'My Song'),
	(1, 'composer', 'Alice,Bob'),
	(1, 'producer', 'Charlie, Dave')
	`)
	if err != nil {
		t.Fatal(err)
	}

	tmpFile := filepath.Join(tmpDir, "output.ttl")
	if err := ExportRDF(dbPath, tmpFile); err != nil {
		t.Fatalf("ExportRDF failed: %v", err)
	}

	fi, err := os.Stat(tmpFile)
	if err != nil {
		t.Fatalf("could not stat RDF output file: %v", err)
	}
	if fi.Size() == 0 {
		t.Errorf("expected non-empty RDF output file")
	}
}

// helper to copy DB (used in older tests if needed)
func copyDB(src, dst string, t *testing.T) {
	srcFile, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer srcFile.Close()
	dstFile, err := os.Create(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer dstFile.Close()
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		t.Fatal(err)
	}
}
