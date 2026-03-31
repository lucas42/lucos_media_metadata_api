package rdfgen

import (
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// Test mapPredicate returns URIs and terms
func TestMapPredicate(t *testing.T) {
	pred, terms := mapPredicate("title", "Song A", "http://localhost:8020")
	if pred != "http://www.w3.org/2004/02/skos/core#prefLabel" {
		t.Errorf("unexpected predicate URI: %s", pred)
	}
	if len(terms) != 1 {
		t.Errorf("expected 1 term, got %d", len(terms))
	}
}

// Test multi-value predicates return a single term per call
func TestMapPredicateMultiValue(t *testing.T) {
	cases := []struct {
		predicateID string
		value       string
		expectedURI string
	}{
		{"composer", "Alice", "http://purl.org/ontology/mo/composer"},
		{"producer", "Charlie", "http://purl.org/ontology/mo/producer"},
		{"language", "en", "http://purl.org/dc/terms/language"},
		{"offence", "violence", "http://localhost:8020/ontology#trigger"},
		{"about", "http://example.com/topic", "http://localhost:8020/ontology#about"},
		{"mentions", "http://example.com/entity", "http://localhost:8020/ontology#mentions"},
	}
	for _, tc := range cases {
		pred, terms := mapPredicate(tc.predicateID, tc.value, "http://localhost:8020")
		if pred != tc.expectedURI {
			t.Errorf("predicate %q: expected URI %q, got %q", tc.predicateID, tc.expectedURI, pred)
		}
		if len(terms) != 1 {
			t.Errorf("predicate %q: expected 1 term, got %d", tc.predicateID, len(terms))
		}
	}
}

// Test exportRDF with temporary DB using separate rows for multi-value fields
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

	// Insert a track and tags — multi-value fields use separate rows
	_, err = db.Exec(`INSERT INTO track (id, url, duration) VALUES (1, 'http://example.com', 120)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
	INSERT INTO tag (trackid, predicateid, value) VALUES
	(1, 'title', 'My Song'),
	(1, 'composer', 'Alice'),
	(1, 'composer', 'Bob'),
	(1, 'producer', 'Charlie'),
	(1, 'producer', 'Dave')
	`)
	if err != nil {
		t.Fatal(err)
	}

	tmpFile := filepath.Join(tmpDir, "output.ttl")
	os.Setenv("MEDIA_METADATA_MANAGER_ORIGIN", "http://localhost:8020")
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
