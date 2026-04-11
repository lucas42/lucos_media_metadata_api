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
	pred, terms := mapPredicate("title", "Song A", nil, "http://localhost:8020")
	if pred != "http://www.w3.org/2004/02/skos/core#prefLabel" {
		t.Errorf("unexpected predicate URI: %s", pred)
	}
	if len(terms) != 1 {
		t.Errorf("expected 1 term, got %d", len(terms))
	}
}

// Test multi-value predicates: each DB row produces one term per call,
// and multiple calls with the same predicate return distinct terms.
// Note: language/about/mentions are excluded here because they require a uri column value
// to produce output — without one they are silently skipped (see TestMapPredicateSkipsWhenNoUri).
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
		{"offence", "violence", "http://localhost:8020/ontology#trigger"},
	}
	// Track terms by predicate to verify distinct values
	termsByPredicate := make(map[string][]string)
	for _, tc := range cases {
		pred, terms := mapPredicate(tc.predicateID, tc.value, nil, "http://localhost:8020")
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
	CREATE TABLE tag (trackid INTEGER, predicateid TEXT, value TEXT, uri TEXT);
	CREATE TABLE album (id INTEGER PRIMARY KEY, name TEXT);
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

// TestExportRDFUsesTagUriForAboutMentions verifies that when a tag has a uri column value,
// that URI is used in the RDF output instead of the display name in value.
// After migrateEolasData, about/mentions tags store the display name in value and
// the actual URI in uri — the RDF export must use the URI.
func TestExportRDFUsesTagUriForAboutMentions(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`
	CREATE TABLE track (id INTEGER PRIMARY KEY, url TEXT, duration INTEGER);
	CREATE TABLE tag (trackid INTEGER, predicateid TEXT, value TEXT, uri TEXT);
	CREATE TABLE album (id INTEGER PRIMARY KEY, name TEXT);
	`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`INSERT INTO track (id, url, duration) VALUES (1, 'http://example.com', 60)`)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate post-migration state: value = display name, uri = actual URI
	_, err = db.Exec(`
	INSERT INTO tag (trackid, predicateid, value, uri) VALUES
	(1, 'about', 'Alice', 'https://eolas.l42.eu/metadata/person/alice/'),
	(1, 'mentions', 'Some Topic', 'https://eolas.l42.eu/metadata/topic/some-topic/')
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

	// URI from tag.uri should appear in output, not the display name
	if !strings.Contains(output, "eolas.l42.eu/metadata/person/alice/") {
		t.Error("expected about URI 'eolas.l42.eu/metadata/person/alice/' in output, but not found")
	}
	if strings.Contains(output, "<Alice>") || strings.Contains(output, "\"Alice\"") {
		t.Error("display name 'Alice' should not appear as an IRI object — only the URI should")
	}
	if !strings.Contains(output, "eolas.l42.eu/metadata/topic/some-topic/") {
		t.Error("expected mentions URI 'eolas.l42.eu/metadata/topic/some-topic/' in output, but not found")
	}
}

// TestMapPredicateLanguageUri verifies that the language case uses the uri field
// when set, and skips the tag when uri is absent.
func TestMapPredicateLanguageUri(t *testing.T) {
	// With uri set: should use uri directly
	uri := "https://eolas.l42.eu/metadata/language/gd/"
	pred, terms := mapPredicate("language", "Scottish Gaelic", &uri, "http://localhost:8020")
	if pred != "http://purl.org/dc/terms/language" {
		t.Errorf("unexpected predicate URI: %s", pred)
	}
	if len(terms) != 1 {
		t.Fatalf("expected 1 term, got %d", len(terms))
	}
	got := terms[0].String()
	if !strings.Contains(got, "eolas.l42.eu/metadata/language/gd/") {
		t.Errorf("expected URI to contain 'eolas.l42.eu/metadata/language/gd/', got: %s", got)
	}
	if strings.Contains(got, "Scottish") {
		t.Errorf("value should not appear in IRI when uri is set, got: %s", got)
	}

	// Without uri: tag is skipped (returns empty)
	pred2, terms2 := mapPredicate("language", "en", nil, "http://localhost:8020")
	if pred2 != "" || len(terms2) != 0 {
		t.Errorf("expected language tag with no uri to be skipped, got pred=%q terms=%v", pred2, terms2)
	}
}

// TestMapPredicateSkipsWhenNoUri verifies that language/about/mentions tags without
// a uri value are silently skipped rather than using value as a fallback IRI.
func TestMapPredicateSkipsWhenNoUri(t *testing.T) {
	for _, predicateID := range []string{"language", "about", "mentions"} {
		pred, terms := mapPredicate(predicateID, "some label", nil, "http://localhost:8020")
		if pred != "" || len(terms) != 0 {
			t.Errorf("predicate %q with nil uri: expected skip (empty pred and terms), got pred=%q terms=%v", predicateID, pred, terms)
		}
		emptyURI := ""
		pred2, terms2 := mapPredicate(predicateID, "some label", &emptyURI, "http://localhost:8020")
		if pred2 != "" || len(terms2) != 0 {
			t.Errorf("predicate %q with empty uri: expected skip (empty pred and terms), got pred=%q terms=%v", predicateID, pred2, terms2)
		}
	}
}

// TestExportRDFSkipsTagsWithNoUri verifies that about/mentions/language tags
// with no uri are silently omitted rather than producing invalid RDF.
func TestExportRDFSkipsTagsWithNoUri(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`
	CREATE TABLE track (id INTEGER PRIMARY KEY, url TEXT, duration INTEGER);
	CREATE TABLE tag (trackid INTEGER, predicateid TEXT, value TEXT, uri TEXT);
	CREATE TABLE album (id INTEGER PRIMARY KEY, name TEXT);
	`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`INSERT INTO track (id, url, duration) VALUES (1, 'http://example.com', 60)`)
	if err != nil {
		t.Fatal(err)
	}
	// Legacy tags with no uri — should be silently omitted
	_, err = db.Exec(`
	INSERT INTO tag (trackid, predicateid, value, uri) VALUES
	(1, 'title', 'My Song', ''),
	(1, 'about', '🌧️ Rain', ''),
	(1, 'mentions', 'Some Label', NULL),
	(1, 'language', 'en', '')
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

	// The title should appear (it's a literal, not URI-dependent)
	if !strings.Contains(output, "My Song") {
		t.Error("expected title 'My Song' in output")
	}
	// The URI-dependent tag values should be absent (no triple should reference them)
	for _, absent := range []string{"Rain", "Some Label"} {
		if strings.Contains(output, absent) {
			t.Errorf("expected %q to be absent from output (tag had no uri), but it was present", absent)
		}
	}
}

// TestMapPredicateAlbumUsesOnAlbum verifies that album tags now map to onAlbum, not dc:isPartOf.
func TestMapPredicateAlbumUsesOnAlbum(t *testing.T) {
	uri := "http://localhost:8020/albums/1"
	pred, terms := mapPredicate("album", "Abbey Road", &uri, "http://localhost:8020")
	if pred != "http://localhost:8020/ontology#onAlbum" {
		t.Errorf("expected onAlbum predicate URI, got %q", pred)
	}
	if len(terms) != 1 {
		t.Fatalf("expected 1 term, got %d", len(terms))
	}
	if !strings.Contains(terms[0].String(), "albums/1") {
		t.Errorf("expected album URI in term, got %q", terms[0].String())
	}
}

// TestAlbumToRdf verifies that AlbumToRdf emits mo:Record type and skos:prefLabel.
func TestAlbumToRdf(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE album (id INTEGER PRIMARY KEY, name TEXT)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO album (id, name) VALUES (1, 'Abbey Road'), (2, 'Let It Be')`)
	if err != nil {
		t.Fatal(err)
	}

	os.Setenv("MEDIA_METADATA_MANAGER_ORIGIN", "http://localhost:8020")

	rows, err := db.Query("SELECT id, name FROM album ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	g, err := AlbumToRdf(rows)
	if err != nil {
		t.Fatalf("AlbumToRdf failed: %v", err)
	}

	var buf strings.Builder
	if err := g.Serialize(&buf, "text/turtle"); err != nil {
		t.Fatalf("serialize failed: %v", err)
	}
	output := buf.String()

	if !strings.Contains(output, "albums/1") {
		t.Error("expected album 1 URI in output")
	}
	if !strings.Contains(output, "albums/2") {
		t.Error("expected album 2 URI in output")
	}
	if !strings.Contains(output, "mo/Record") {
		t.Errorf("expected mo:Record type in output, got:\n%s", output)
	}
	if !strings.Contains(output, "Abbey Road") {
		t.Error("expected album name 'Abbey Road' in output")
	}
	if !strings.Contains(output, "Let It Be") {
		t.Error("expected album name 'Let It Be' in output")
	}
}

// TestExportRDFIncludesAlbums verifies that ExportRDF emits album triples alongside tracks.
func TestExportRDFIncludesAlbums(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`
	CREATE TABLE track (id INTEGER PRIMARY KEY, url TEXT, duration INTEGER);
	CREATE TABLE tag (trackid INTEGER, predicateid TEXT, value TEXT, uri TEXT);
	CREATE TABLE album (id INTEGER PRIMARY KEY, name TEXT);
	`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO track (id, url, duration) VALUES (1, 'http://example.com', 120)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO album (id, name) VALUES (1, 'Abbey Road')`)
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

	if !strings.Contains(output, "Abbey Road") {
		t.Error("expected album name 'Abbey Road' in export output")
	}
	if !strings.Contains(output, "albums/1") {
		t.Error("expected album URI in export output")
	}
	if !strings.Contains(output, "mo/Record") {
		t.Error("expected mo:Record type in export output")
	}
}

// TestOntologyToRdfIncludesMoRecordAndOnAlbum verifies OntologyToRdf emits mo:Record metadata and onAlbum property.
func TestOntologyToRdfIncludesMoRecordAndOnAlbum(t *testing.T) {
	os.Setenv("MEDIA_METADATA_MANAGER_ORIGIN", "http://localhost:8020")

	g, err := OntologyToRdf()
	if err != nil {
		t.Fatalf("OntologyToRdf failed: %v", err)
	}

	var buf strings.Builder
	if err := g.Serialize(&buf, "text/turtle"); err != nil {
		t.Fatalf("serialize failed: %v", err)
	}
	output := buf.String()

	if !strings.Contains(output, "mo/Record") {
		t.Error("expected mo:Record in ontology output")
	}
	if !strings.Contains(output, "Album") {
		t.Errorf("expected 'Album' prefLabel in ontology output, got:\n%s", output)
	}
	if !strings.Contains(output, "onAlbum") {
		t.Error("expected onAlbum property in ontology output")
	}
	if !strings.Contains(output, "mo/track") {
		t.Error("expected mo:track inverseOf reference in ontology output")
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
