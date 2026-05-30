package rdfgen

import (
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rdf2go "github.com/deiu/rdf2go"
	_ "github.com/mattn/go-sqlite3"
)

// Test mapPredicate returns URIs and terms
func TestMapPredicate(t *testing.T) {
	pred, terms := mapPredicate("title", "Song A", nil, "http://localhost:8020", "http://localhost:3002")
	if pred != "http://www.w3.org/2004/02/skos/core#prefLabel" {
		t.Errorf("unexpected predicate URI: %s", pred)
	}
	if len(terms) != 1 {
		t.Errorf("expected 1 term, got %d", len(terms))
	}
}

// Test multi-value predicates: each DB row produces one term per call,
// and multiple calls with the same predicate return distinct terms.
// composer and producer are now URIObject predicates — they require a uri column
// value and are silently skipped when absent (see TestMapPredicateSkipsWhenNoUri).
// language/about/mentions/offence/composer/producer are therefore tested here
// using their URI-object form.
func TestMapPredicateMultiValue(t *testing.T) {
	aliceURI := "https://eolas.l42.eu/metadata/person/alice/"
	bobURI := "https://eolas.l42.eu/metadata/person/bob/"
	charlieURI := "https://eolas.l42.eu/metadata/person/charlie/"
	daveURI := "https://eolas.l42.eu/metadata/person/dave/"
	cases := []struct {
		predicateID string
		value       string
		uri         *string
		expectedURI string
	}{
		{"composer", "Alice", &aliceURI, "http://purl.org/ontology/mo/composer"},
		{"composer", "Bob", &bobURI, "http://purl.org/ontology/mo/composer"},
		{"producer", "Charlie", &charlieURI, "http://purl.org/ontology/mo/producer"},
		{"producer", "Dave", &daveURI, "http://purl.org/ontology/mo/producer"},
	}
	// Track terms by predicate to verify distinct values
	termsByPredicate := make(map[string][]string)
	for _, tc := range cases {
		pred, terms := mapPredicate(tc.predicateID, tc.value, tc.uri, "http://localhost:8020", "http://localhost:3002")
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
	CREATE TABLE artist (id INTEGER PRIMARY KEY, name TEXT);
	`)
	if err != nil {
		t.Fatal(err)
	}

	// Insert a track and tags — multi-value fields use separate rows.
	// composer and producer are URIObject predicates and require a uri column value.
	_, err = db.Exec(`INSERT INTO track (id, url, duration) VALUES (1, 'http://example.com', 120)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
	INSERT INTO tag (trackid, predicateid, value, uri) VALUES
	(1, 'title', 'My Song', ''),
	(1, 'composer', 'Alice', 'https://eolas.l42.eu/metadata/person/alice/'),
	(1, 'composer', 'Bob', 'https://eolas.l42.eu/metadata/person/bob/'),
	(1, 'producer', 'Charlie', 'https://eolas.l42.eu/metadata/person/charlie/'),
	(1, 'producer', 'Dave', 'https://eolas.l42.eu/metadata/person/dave/')
	`)
	if err != nil {
		t.Fatal(err)
	}

	tmpFile := filepath.Join(tmpDir, "output.ttl")
	os.Setenv("MEDIA_METADATA_MANAGER_ORIGIN", "http://localhost:8020")
	os.Setenv("APP_ORIGIN", "http://localhost:3002")
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

	// Verify all multi-value tags appear in the output.
	// composer/producer now emit eolas Person URIs, not freetext names.
	for _, expected := range []string{
		"My Song",
		"eolas.l42.eu/metadata/person/alice/",
		"eolas.l42.eu/metadata/person/bob/",
		"eolas.l42.eu/metadata/person/charlie/",
		"eolas.l42.eu/metadata/person/dave/",
	} {
		if !strings.Contains(output, expected) {
			t.Errorf("expected RDF output to contain %q, but it was missing", expected)
		}
	}
}

// TestExportRDFRejectsEmptyTrackTable verifies that ExportRDF returns an error rather than
// writing an empty file when the track table contains no rows (sanity floor check).
func TestExportRDFRejectsEmptyTrackTable(t *testing.T) {
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
	CREATE TABLE artist (id INTEGER PRIMARY KEY, name TEXT);
	`)
	if err != nil {
		t.Fatal(err)
	}
	// Intentionally empty track table — exporter must refuse to write output.

	outFile := filepath.Join(tmpDir, "output.ttl")
	os.Setenv("MEDIA_METADATA_MANAGER_ORIGIN", "http://localhost:8020")
	os.Setenv("APP_ORIGIN", "http://localhost:3002")
	err = ExportRDF(dbPath, outFile)
	if err == nil {
		t.Fatal("expected ExportRDF to return an error for an empty track table, but it returned nil")
	}
	if _, statErr := os.Stat(outFile); !os.IsNotExist(statErr) {
		t.Error("expected output file NOT to be written when the sanity check fails")
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
	CREATE TABLE artist (id INTEGER PRIMARY KEY, name TEXT);
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
	os.Setenv("APP_ORIGIN", "http://localhost:3002")
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
	pred, terms := mapPredicate("language", "Scottish Gaelic", &uri, "http://localhost:8020", "http://localhost:3002")
	if pred != "http://localhost:3002/ontology#trackLanguage" {
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
	pred2, terms2 := mapPredicate("language", "en", nil, "http://localhost:8020", "http://localhost:3002")
	if pred2 != "" || len(terms2) != 0 {
		t.Errorf("expected language tag with no uri to be skipped, got pred=%q terms=%v", pred2, terms2)
	}
}

// TestMapPredicateOffenceUri verifies that offence tags use the uri field (URI-object pattern),
// skip when no uri is set, and emit the correct trigger predicate.
func TestMapPredicateOffenceUri(t *testing.T) {
	offenceURI := "https://eolas.l42.eu/metadata/offence/3/"
	pred, terms := mapPredicate("offence", "violence", &offenceURI, "http://localhost:8020", "http://localhost:3002")
	if pred != "http://localhost:3002/ontology#trigger" {
		t.Errorf("expected trigger predicate, got %q", pred)
	}
	if len(terms) != 1 {
		t.Fatalf("expected 1 term, got %d", len(terms))
	}
	if !strings.Contains(terms[0].String(), "eolas.l42.eu/metadata/offence/3/") {
		t.Errorf("expected term to contain eolas offence URI, got %q", terms[0].String())
	}

	// Without a URI, offence tags are silently skipped (legacy data).
	pred2, terms2 := mapPredicate("offence", "violence", nil, "http://localhost:8020", "http://localhost:3002")
	if pred2 != "" || len(terms2) != 0 {
		t.Errorf("expected offence with nil uri to be skipped, got pred=%q terms=%v", pred2, terms2)
	}
}

// TestMapPredicateSkipsWhenNoUri verifies that all URIObject predicates are silently
// skipped when uri is absent.
func TestMapPredicateSkipsWhenNoUri(t *testing.T) {
	for _, predicateID := range []string{"language", "about", "mentions", "offence", "composer", "producer", "artist"} {
		pred, terms := mapPredicate(predicateID, "some label", nil, "http://localhost:8020", "http://localhost:3002")
		if pred != "" || len(terms) != 0 {
			t.Errorf("predicate %q with nil uri: expected skip (empty pred and terms), got pred=%q terms=%v", predicateID, pred, terms)
		}
		emptyURI := ""
		pred2, terms2 := mapPredicate(predicateID, "some label", &emptyURI, "http://localhost:8020", "http://localhost:3002")
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
	CREATE TABLE artist (id INTEGER PRIMARY KEY, name TEXT);
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
	os.Setenv("APP_ORIGIN", "http://localhost:3002")
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

// TestMapPredicateArtistUsesURI verifies that artist tags now use the URIObject
// shape (foaf:maker predicate, tag.uri as the IRI object), not a search URL.
func TestMapPredicateArtistUsesURI(t *testing.T) {
	artistURI := "http://localhost:8020/artists/1"
	pred, terms := mapPredicate("artist", "Enya", &artistURI, "http://localhost:8020", "http://localhost:3002")
	if pred != "http://xmlns.com/foaf/0.1/maker" {
		t.Errorf("expected foaf:maker predicate URI, got %q", pred)
	}
	if len(terms) != 1 {
		t.Fatalf("expected 1 term, got %d", len(terms))
	}
	if !strings.Contains(terms[0].String(), "artists/1") {
		t.Errorf("expected artist URI in term, got %q", terms[0].String())
	}
}

// TestMapPredicateMBIDPrefixPredicates verifies that all 3 MBID predicates are routed
// via the registry (ValueShapeMBIDPrefix) and produce IRI objects with the correct prefix.
func TestMapPredicateMBIDPrefixPredicates(t *testing.T) {
	mbid := "550e8400-e29b-41d4-a716-446655440000"
	cases := []struct {
		predicateID  string
		expectedPred string
		expectedBase string
	}{
		{"mbid_artist", "http://purl.org/dc/terms/creator", "https://musicbrainz.org/artist/"},
		{"mbid_recording", "http://purl.org/dc/terms/identifier", "https://musicbrainz.org/recording/"},
		{"mbid_release", "http://purl.org/dc/terms/isPartOf", "https://musicbrainz.org/release/"},
	}
	for _, tc := range cases {
		pred, terms := mapPredicate(tc.predicateID, mbid, nil, "http://localhost:8020", "http://localhost:3002")
		if pred != tc.expectedPred {
			t.Errorf("predicate %q: expected predicate URI %q, got %q", tc.predicateID, tc.expectedPred, pred)
		}
		if len(terms) != 1 {
			t.Errorf("predicate %q: expected 1 term, got %d", tc.predicateID, len(terms))
			continue
		}
		termStr := terms[0].String()
		expectedIRI := tc.expectedBase + mbid
		if !strings.Contains(termStr, expectedIRI) {
			t.Errorf("predicate %q: expected term to contain %q, got %q", tc.predicateID, expectedIRI, termStr)
		}
	}
}

// TestMapPredicateUnknownOmitted verifies that predicates not in the registry are silently
// omitted rather than emitting an orphan ${APP_ORIGIN}/ontology#X literal triple.
func TestMapPredicateUnknownOmitted(t *testing.T) {
	pred, terms := mapPredicate("totally_unknown_predicate_xyz", "somevalue", nil, "http://localhost:8020", "http://localhost:3002")
	if pred != "" || len(terms) != 0 {
		t.Errorf("expected unknown predicate to be omitted (empty pred and nil terms), got pred=%q terms=%v", pred, terms)
	}
}

// TestMapPredicateExplicitOmitProducesNoTriples verifies that predicates registered
// with ValueShapeOmit produce no RDF triples.
func TestMapPredicateExplicitOmitProducesNoTriples(t *testing.T) {
	for _, predicateID := range []string{"genre"} {
		pred, terms := mapPredicate(predicateID, "somevalue", nil, "http://localhost:8020", "http://localhost:3002")
		if pred != "" || len(terms) != 0 {
			t.Errorf("predicate %q: expected Omit to produce no triples, got pred=%q terms=%v", predicateID, pred, terms)
		}
	}
}

// TestMapPredicateSKOSPredicatesWithUri verifies that provenance, availability, singalong, and dance
// emit correct IRI triples when a concept URI is provided (ValueShapeURIObject).
func TestMapPredicateSKOSPredicatesWithUri(t *testing.T) {
	cases := []struct {
		predicateID  string
		uri          string
		expectedPred string
	}{
		{"provenance", "http://localhost:3002/vocab/provenance/bandcamp", "http://purl.org/dc/terms/source"},
		{"availability", "http://localhost:3002/vocab/availability/4", "http://localhost:3002/ontology#availability"},
		{"singalong", "http://localhost:3002/vocab/singalong/0", "http://localhost:3002/ontology#singalong"},
		{"dance", "http://localhost:3002/vocab/dance/lindy-hop", "http://localhost:3002/ontology#dance"},
	}
	for _, tc := range cases {
		uri := tc.uri
		pred, terms := mapPredicate(tc.predicateID, "some label", &uri, "http://localhost:8020", "http://localhost:3002")
		if pred != tc.expectedPred {
			t.Errorf("predicate %q: expected predicate URI %q, got %q", tc.predicateID, tc.expectedPred, pred)
		}
		if len(terms) != 1 {
			t.Errorf("predicate %q: expected 1 term, got %d", tc.predicateID, len(terms))
			continue
		}
		if !strings.Contains(terms[0].String(), tc.uri) {
			t.Errorf("predicate %q: expected term to contain URI %q, got %q", tc.predicateID, tc.uri, terms[0].String())
		}
	}
}

// TestMapPredicateSKOSPredicatesSkipWhenNoUri verifies that SKOS predicates without a URI
// are silently skipped (matching the general URIObject behaviour).
func TestMapPredicateSKOSPredicatesSkipWhenNoUri(t *testing.T) {
	for _, predicateID := range []string{"provenance", "availability", "singalong", "dance"} {
		pred, terms := mapPredicate(predicateID, "some label", nil, "http://localhost:8020", "http://localhost:3002")
		if pred != "" || len(terms) != 0 {
			t.Errorf("predicate %q with nil uri: expected skip, got pred=%q terms=%v", predicateID, pred, terms)
		}
	}
}

// TestGenreOmitProducesNoTriples verifies that genre is now Omit (not SearchURL).
func TestGenreOmitProducesNoTriples(t *testing.T) {
	pred, terms := mapPredicate("genre", "rock", nil, "http://localhost:8020", "http://localhost:3002")
	if pred != "" || len(terms) != 0 {
		t.Errorf("genre: expected Omit to produce no triples, got pred=%q terms=%v", pred, terms)
	}
}

// TestMapPredicateAlbumUsesOnAlbum verifies that album tags now map to onAlbum, not dc:isPartOf.
func TestMapPredicateAlbumUsesOnAlbum(t *testing.T) {
	uri := "http://localhost:8020/albums/1"
	pred, terms := mapPredicate("album", "Abbey Road", &uri, "http://localhost:8020", "http://localhost:3002")
	if pred != "http://localhost:3002/ontology#onAlbum" {
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
	// Type-level metadata for mo:Record must be inline so the document is self-contained
	if !strings.Contains(output, "Album") {
		t.Errorf("expected mo:Record prefLabel 'Album' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "hasCategory") {
		t.Errorf("expected eolas:hasCategory triple for mo:Record in output, got:\n%s", output)
	}
	if !strings.Contains(output, "Musical") {
		t.Errorf("expected eolas:Musical in output, got:\n%s", output)
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
	CREATE TABLE artist (id INTEGER PRIMARY KEY, name TEXT);
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
	os.Setenv("APP_ORIGIN", "http://localhost:3002")
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
	os.Setenv("APP_ORIGIN", "http://localhost:3002")

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

// TestOntologyToRdfIncludesMoTrackPrefLabel verifies OntologyToRdf emits skos:prefLabel for mo:track.
// Without this triple, clients that rely on skos:prefLabel cannot display the album→tracks relationship.
func TestOntologyToRdfIncludesMoTrackPrefLabel(t *testing.T) {
	os.Setenv("MEDIA_METADATA_MANAGER_ORIGIN", "http://localhost:8020")
	os.Setenv("APP_ORIGIN", "http://localhost:3002")

	g, err := OntologyToRdf()
	if err != nil {
		t.Fatalf("OntologyToRdf failed: %v", err)
	}

	moTrackProperty := rdf2go.NewResource("http://purl.org/ontology/mo/track")
	prefLabel := rdf2go.NewResource("http://www.w3.org/2004/02/skos/core#prefLabel")
	expectedLabel := rdf2go.NewLiteralWithLanguage("Track", "en")

	triples := g.All(moTrackProperty, prefLabel, nil)
	if len(triples) == 0 {
		t.Error("expected skos:prefLabel triple for mo:track, got none")
		return
	}
	if triples[0].Object.String() != expectedLabel.String() {
		t.Errorf("expected mo:track skos:prefLabel to be %q, got %q", expectedLabel, triples[0].Object)
	}
}

// TestExportRDFTrackLanguageEmission verifies that a track with a language tag
// emits mmm:trackLanguage pointing to the language URI.
func TestExportRDFTrackLanguageEmission(t *testing.T) {
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
	CREATE TABLE artist (id INTEGER PRIMARY KEY, name TEXT);
	`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO track (id, url, duration) VALUES (1, 'http://example.com', 180)`)
	if err != nil {
		t.Fatal(err)
	}
	langURI := "https://eolas.l42.eu/metadata/language/gd/"
	_, err = db.Exec(`INSERT INTO tag (trackid, predicateid, value, uri) VALUES (1, 'language', 'Scottish Gaelic', ?)`, langURI)
	if err != nil {
		t.Fatal(err)
	}

	tmpFile := filepath.Join(tmpDir, "output.ttl")
	os.Setenv("MEDIA_METADATA_MANAGER_ORIGIN", "http://localhost:8020")
	os.Setenv("APP_ORIGIN", "http://localhost:3002")
	if err := ExportRDF(dbPath, tmpFile); err != nil {
		t.Fatalf("ExportRDF failed: %v", err)
	}
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("could not read RDF output file: %v", err)
	}
	output := string(content)

	// mmm:trackLanguage must be present; dcterms:language must not be emitted
	if !strings.Contains(output, "ontology#trackLanguage") {
		t.Error("expected mmm:trackLanguage triple in output")
	}
	if strings.Contains(output, "dc/terms/language") {
		t.Error("dcterms:language must not be emitted — mmm:trackLanguage is the only language predicate")
	}
	// Must point to the language URI
	if !strings.Contains(output, "eolas.l42.eu/metadata/language/gd/") {
		t.Error("expected language URI 'eolas.l42.eu/metadata/language/gd/' in output")
	}
}

// TestTrackLanguageNoUriNoTrackLanguageTriple verifies that a language tag with
// no URI emits no mmm:trackLanguage triple on the track subject.
// We query the TrackToRdf graph directly rather than string-matching the full
// serialized output, because OntologyToRdf (merged by ExportRDF) legitimately
// declares trackLanguage as a property URI.
func TestTrackLanguageNoUriNoTrackLanguageTriple(t *testing.T) {
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
	`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO track (id, url, duration) VALUES (1, 'http://example.com', 60)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO tag (trackid, predicateid, value, uri) VALUES (1, 'language', 'en', '')`)
	if err != nil {
		t.Fatal(err)
	}

	os.Setenv("MEDIA_METADATA_MANAGER_ORIGIN", "http://localhost:8020")
	os.Setenv("APP_ORIGIN", "http://localhost:3002")
	rows, err := db.Query(`
		SELECT t.id, t.url, t.duration, tg.predicateid, tg.value, tg.uri
		FROM track t
		LEFT JOIN tag tg ON tg.trackid = t.id
		ORDER BY t.id
	`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	g, _, err := TrackToRdf(rows)
	if err != nil {
		t.Fatalf("TrackToRdf failed: %v", err)
	}

	// No mmm:trackLanguage triple expected on the track subject
	trackLang := rdf2go.NewResource("http://localhost:3002/ontology#trackLanguage")
	if triples := g.All(nil, trackLang, nil); len(triples) > 0 {
		t.Errorf("expected no mmm:trackLanguage triple when language tag has no URI, got %d", len(triples))
	}
}

// TestOntologyToRdfIncludesTrackLanguage verifies that OntologyToRdf emits the
// trackLanguage property, its inverse trackInLanguage, and the correct prefLabels.
func TestOntologyToRdfIncludesTrackLanguage(t *testing.T) {
	os.Setenv("MEDIA_METADATA_MANAGER_ORIGIN", "http://localhost:8020")
	os.Setenv("APP_ORIGIN", "http://localhost:3002")

	g, err := OntologyToRdf()
	if err != nil {
		t.Fatalf("OntologyToRdf failed: %v", err)
	}

	var buf strings.Builder
	if err := g.Serialize(&buf, "text/turtle"); err != nil {
		t.Fatalf("serialize failed: %v", err)
	}
	output := buf.String()

	if !strings.Contains(output, "trackLanguage") {
		t.Error("expected trackLanguage property in ontology output")
	}
	if !strings.Contains(output, "trackInLanguage") {
		t.Error("expected trackInLanguage inverse property in ontology output")
	}
	if !strings.Contains(output, "Track Language") {
		t.Error("expected prefLabel 'Track Language' in ontology output")
	}
	if !strings.Contains(output, "Tracks in this language") {
		t.Error("expected prefLabel 'Tracks in this language' in ontology output")
	}
	if !strings.Contains(output, "inverseOf") {
		t.Error("expected owl:inverseOf declaration in ontology output")
	}
	if !strings.Contains(output, "eolas.l42.eu/metadata/language/") {
		t.Error("expected range URI 'eolas.l42.eu/metadata/language/' in ontology output")
	}
}

// TestOntologyToRdfIncludesSKOSSchemes verifies that OntologyToRdf includes the SKOS
// concept schemes for provenance, availability, singalong, and dance.
func TestOntologyToRdfIncludesSKOSSchemes(t *testing.T) {
	os.Setenv("APP_ORIGIN", "http://localhost:3002")
	t.Cleanup(func() { os.Unsetenv("APP_ORIGIN") })

	g, err := OntologyToRdf()
	if err != nil {
		t.Fatalf("OntologyToRdf failed: %v", err)
	}

	var buf strings.Builder
	if err := g.Serialize(&buf, "text/turtle"); err != nil {
		t.Fatalf("serialize failed: %v", err)
	}
	output := buf.String()

	// Concept scheme declarations
	for _, scheme := range []string{"provenanceScheme", "availabilityScheme", "singalongScheme", "danceScheme"} {
		if !strings.Contains(output, scheme) {
			t.Errorf("expected %q in ontology output", scheme)
		}
	}

	// Ordinal level properties
	if !strings.Contains(output, "availabilityLevel") {
		t.Error("expected availabilityLevel property in ontology output")
	}
	if !strings.Contains(output, "singalongLevel") {
		t.Error("expected singalongLevel property in ontology output")
	}

	// Sample concept URIs from each predicate
	samples := []string{
		"vocab/provenance/bandcamp",
		"vocab/availability/4",
		"vocab/singalong/0",
		"vocab/dance/lindy-hop",
	}
	for _, sample := range samples {
		if !strings.Contains(output, sample) {
			t.Errorf("expected %q in ontology output", sample)
		}
	}

	// Dance concepts should be present
	if !strings.Contains(output, "vocab/dance/lindy-hop") {
		t.Error("expected dance concept URIs in ontology output")
	}
}

// TestOntologyToRdfSingalongAndDanceProperties verifies the new singalong and dance
// property declarations appear in the ontology output.
func TestOntologyToRdfSingalongAndDanceProperties(t *testing.T) {
	os.Setenv("APP_ORIGIN", "http://localhost:3002")
	t.Cleanup(func() { os.Unsetenv("APP_ORIGIN") })

	g, err := OntologyToRdf()
	if err != nil {
		t.Fatalf("OntologyToRdf failed: %v", err)
	}

	var buf strings.Builder
	if err := g.Serialize(&buf, "text/turtle"); err != nil {
		t.Fatalf("serialize failed: %v", err)
	}
	output := buf.String()

	if !strings.Contains(output, "ontology#singalong") {
		t.Error("expected singalong property definition in ontology output")
	}
	if !strings.Contains(output, "ontology#dance") {
		t.Error("expected dance property definition in ontology output")
	}
}

// TestExportRDFSKOSConceptEmission verifies that track tags with SKOS concept URIs are
// correctly emitted as IRI triples in the RDF export.
func TestExportRDFSKOSConceptEmission(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`
	CREATE TABLE track (id INTEGER PRIMARY KEY, url TEXT, duration INTEGER);
	CREATE TABLE tag (trackid INTEGER, predicateid TEXT, value TEXT, uri TEXT);
	CREATE TABLE album (id INTEGER PRIMARY KEY, name TEXT);
	CREATE TABLE artist (id INTEGER PRIMARY KEY, name TEXT);
	`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO track (id, url, duration) VALUES (1, 'http://example.com', 180)`)
	if err != nil {
		t.Fatal(err)
	}

	// Insert post-migration SKOS tags: value = prefLabel, uri = concept URI
	_, err = db.Exec(`
	INSERT INTO tag (trackid, predicateid, value, uri) VALUES
	(1, 'provenance', 'Bandcamp', 'http://localhost:3002/vocab/provenance/bandcamp'),
	(1, 'availability', 'Ubiquitous', 'http://localhost:3002/vocab/availability/4'),
	(1, 'singalong', 'No chance', 'http://localhost:3002/vocab/singalong/0'),
	(1, 'dance', 'Lindy Hop', 'http://localhost:3002/vocab/dance/lindy-hop')
	`)
	if err != nil {
		t.Fatal(err)
	}

	tmpFile := tmpDir + "/output.ttl"
	os.Setenv("MEDIA_METADATA_MANAGER_ORIGIN", "http://localhost:8020")
	os.Setenv("APP_ORIGIN", "http://localhost:3002")
	if err := ExportRDF(dbPath, tmpFile); err != nil {
		t.Fatalf("ExportRDF failed: %v", err)
	}

	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("could not read RDF output file: %v", err)
	}
	output := string(content)

	// Each concept URI should appear as an IRI object in the output
	expectedURIs := []string{
		"vocab/provenance/bandcamp",
		"vocab/availability/4",
		"vocab/singalong/0",
		"vocab/dance/lindy-hop",
	}
	for _, uri := range expectedURIs {
		if !strings.Contains(output, uri) {
			t.Errorf("expected concept URI %q in RDF output", uri)
		}
	}
}

// TestArtistToRdf verifies that ArtistToRdf emits mo:MusicArtist type, skos:prefLabel,
// and the critical type-level metadata including the foaf:Agent parent class label.
func TestArtistToRdf(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE artist (id INTEGER PRIMARY KEY, name TEXT)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO artist (id, name) VALUES (1, 'Enya'), (2, 'Clannad')`)
	if err != nil {
		t.Fatal(err)
	}

	os.Setenv("MEDIA_METADATA_MANAGER_ORIGIN", "http://localhost:8020")

	rows, err := db.Query("SELECT id, name FROM artist ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	g, err := ArtistToRdf(rows)
	if err != nil {
		t.Fatalf("ArtistToRdf failed: %v", err)
	}

	var buf strings.Builder
	if err := g.Serialize(&buf, "text/turtle"); err != nil {
		t.Fatalf("serialize failed: %v", err)
	}
	output := buf.String()

	// Entity-level triples
	if !strings.Contains(output, "artists/1") {
		t.Error("expected artist 1 URI in output")
	}
	if !strings.Contains(output, "artists/2") {
		t.Error("expected artist 2 URI in output")
	}
	if !strings.Contains(output, "mo/MusicArtist") {
		t.Errorf("expected mo:MusicArtist type in output, got:\n%s", output)
	}
	if !strings.Contains(output, "Enya") {
		t.Error("expected artist name 'Enya' in output")
	}
	if !strings.Contains(output, "Clannad") {
		t.Error("expected artist name 'Clannad' in output")
	}
	// Type-level metadata
	if !strings.Contains(output, "Artist") {
		t.Errorf("expected mo:MusicArtist prefLabel 'Artist' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "subClassOf") {
		t.Errorf("expected rdfs:subClassOf triple in output, got:\n%s", output)
	}
	if !strings.Contains(output, "foaf/0.1/Agent") {
		t.Errorf("expected foaf:Agent URI in output, got:\n%s", output)
	}
	// foaf:Agent MUST have a prefLabel so arachne's ingestor doesn't fail
	foafAgent := rdf2go.NewResource("http://xmlns.com/foaf/0.1/Agent")
	prefLabelPred := rdf2go.NewResource("http://www.w3.org/2004/02/skos/core#prefLabel")
	agentLabels := g.All(foafAgent, prefLabelPred, nil)
	if len(agentLabels) == 0 {
		t.Error("expected foaf:Agent to have a skos:prefLabel triple (required by arachne ADR-0004)")
	}
	if !strings.Contains(output, "hasCategory") {
		t.Errorf("expected eolas:hasCategory triple for mo:MusicArtist in output, got:\n%s", output)
	}
}

// TestExportRDFIncludesArtists verifies that ExportRDF emits artist triples alongside
// tracks, including both entity-level triples (URI, rdf:type, prefLabel) and the
// type-level metadata for mo:MusicArtist and foaf:Agent.
//
// This is a regression test for the pattern identified in lucos_eolas#273: tests
// written against entity model methods alone can miss bugs in the bulk export path.
// This test exercises ExportRDF directly (the actual bulk export code path) rather
// than calling ArtistToRdf in isolation.
func TestExportRDFIncludesArtists(t *testing.T) {
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
	CREATE TABLE artist (id INTEGER PRIMARY KEY, name TEXT);
	`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO track (id, url, duration) VALUES (1, 'http://example.com', 120)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO artist (id, name) VALUES (1, 'Enya')`)
	if err != nil {
		t.Fatal(err)
	}
	// Tag the track with an artist URI reference
	_, err = db.Exec(`INSERT INTO tag (trackid, predicateid, value, uri) VALUES (1, 'artist', 'Enya', 'http://localhost:8020/artists/1')`)
	if err != nil {
		t.Fatal(err)
	}

	tmpFile := filepath.Join(tmpDir, "output.ttl")
	os.Setenv("MEDIA_METADATA_MANAGER_ORIGIN", "http://localhost:8020")
	os.Setenv("APP_ORIGIN", "http://localhost:3002")
	if err := ExportRDF(dbPath, tmpFile); err != nil {
		t.Fatalf("ExportRDF failed: %v", err)
	}

	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("could not read RDF output file: %v", err)
	}
	output := string(content)

	// Entity-level triples
	if !strings.Contains(output, "artists/1") {
		t.Error("expected artist URI in export output")
	}
	if !strings.Contains(output, "Enya") {
		t.Error("expected artist name 'Enya' in export output")
	}
	if !strings.Contains(output, "mo/MusicArtist") {
		t.Error("expected mo:MusicArtist type in export output")
	}
	// Track-to-artist link via foaf:maker
	if !strings.Contains(output, "foaf/0.1/maker") {
		t.Error("expected foaf:maker predicate in export output")
	}
	// Type-level metadata — mo:MusicArtist prefLabel required by arachne ADR-0004
	if !strings.Contains(output, "subClassOf") {
		t.Error("expected rdfs:subClassOf triple for mo:MusicArtist in export output")
	}
	if !strings.Contains(output, "foaf/0.1/Agent") {
		t.Error("expected foaf:Agent URI in export output (parent class of mo:MusicArtist)")
	}
	// foaf:Agent skos:prefLabel is the critical regression guard: arachne's ingestor
	// calls get_label() on every class in the subclass chain and will fail with
	// ValueError if foaf:Agent has no label in the exported RDF.
	if !strings.Contains(output, "Agent") {
		t.Error("expected foaf:Agent prefLabel 'Agent' in export output")
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
