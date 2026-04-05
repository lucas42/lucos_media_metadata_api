package rdfgen

import (
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"strings"
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

// Test multi-value predicates: each DB row produces one term per call,
// and multiple calls with the same predicate return distinct terms.
func TestMapPredicateMultiValue(t *testing.T) {
	cases := []struct {
		predicateID string
		value       string
		expectedURI string
	}{
		{"composer", "Alice", "http://purl.org/ontology/mo/composer"},
		{"composer", "Bob", "http://purl.org/ontology/mo/composer"},
		{"producer", "Charlie", "http://purl.org/ontology/mo/producer"},
		{"producer", "Dave", "http://purl.org/ontology/mo/producer"},
		{"language", "en", "http://purl.org/dc/terms/language"},
		{"offence", "violence", "http://localhost:8020/ontology#trigger"},
		{"about", "http://example.com/topic", "http://localhost:8020/ontology#about"},
		{"mentions", "http://example.com/entity", "http://localhost:8020/ontology#mentions"},
	}
	// Track terms by predicate to verify distinct values
	termsByPredicate := make(map[string][]string)
	for _, tc := range cases {
		pred, terms := mapPredicate(tc.predicateID, tc.value, "http://localhost:8020")
		if pred != tc.expectedURI {
			t.Errorf("predicate %q value %q: expected URI %q, got %q", tc.predicateID, tc.value, tc.expectedURI, pred)
		}
		if len(terms) != 1 {
			t.Errorf("predicate %q value %q: expected 1 term, got %d", tc.predicateID, tc.value, len(terms))
		}
		if len(terms) > 0 {
			termsByPredicate[tc.predicateID] = append(termsByPredicate[tc.predicateID], terms[0].String())
		}
	}
	// Verify multi-value predicates produce distinct terms
	for _, pred := range []string{"composer", "producer"} {
		terms := termsByPredicate[pred]
		if len(terms) != 2 {
			t.Errorf("predicate %q: expected 2 distinct terms, got %d", pred, len(terms))
		}
		if len(terms) == 2 && terms[0] == terms[1] {
			t.Errorf("predicate %q: both terms are identical (%q) — expected distinct values for each DB row", pred, terms[0])
		}
	}
}

// Test exportRDF with temporary DB using separate rows for multi-value fields.
// Verifies that all values for multi-value predicates appear in the output.
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

	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("could not read RDF output file: %v", err)
	}
	if len(content) == 0 {
		t.Fatalf("expected non-empty RDF output file")
	}

	output := string(content)

	// Verify all multi-value tags appear in the output
	// Each value should generate a separate triple
	for _, expected := range []string{"Alice", "Bob", "Charlie", "Dave", "My Song"} {
		if !strings.Contains(output, expected) {
			t.Errorf("expected RDF output to contain %q, but it was missing", expected)
		}
	}
}

// TestExportRDFEncodesSpacesInIRIs verifies that about/mentions tag values with spaces
// are percent-encoded in Turtle IRI output (%20), not emitted as literal spaces.
func TestExportRDFEncodesSpacesInIRIs(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`
	CREATE TABLE track (id INTEGER PRIMARY KEY, url TEXT, duration INTEGER);
	CREATE TABLE tag (trackid INTEGER, predicateid TEXT, value TEXT);
	`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`INSERT INTO track (id, url, duration) VALUES (1, 'http://example.com', 60)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
	INSERT INTO tag (trackid, predicateid, value) VALUES
	(1, 'about', 'https://eolas.l42.eu/thing/Some Topic'),
	(1, 'mentions', 'https://eolas.l42.eu/thing/Another Entity')
	`)
	if err != nil {
		t.Fatal(err)
	}

	tmpFile := filepath.Join(tmpDir, "output.ttl")
	os.Setenv("MEDIA_METADATA_MANAGER_ORIGIN", "http://localhost:8020")
	if err := ExportRDF(dbPath, tmpFile); err != nil {
		t.Fatalf("ExportRDF failed: %v", err)
	}

	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("could not read RDF output file: %v", err)
	}
	output := string(content)

	// Spaces must be percent-encoded in IRIs
	if strings.Contains(output, "Some Topic") {
		t.Error("expected 'Some Topic' to be encoded as 'Some%20Topic' in Turtle IRI, but found literal space")
	}
	if !strings.Contains(output, "Some%20Topic") {
		t.Error("expected 'Some%20Topic' in Turtle output for 'about' tag, but not found")
	}
	if strings.Contains(output, "Another Entity") {
		t.Error("expected 'Another Entity' to be encoded as 'Another%20Entity' in Turtle IRI, but found literal space")
	}
	if !strings.Contains(output, "Another%20Entity") {
		t.Error("expected 'Another%20Entity' in Turtle output for 'mentions' tag, but not found")
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
