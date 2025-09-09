package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRDFHandler_ResourcesAndLiterals(t *testing.T) {
	// Create in-memory DB
	store := DBInit(":memory:", nil)

	// Insert test track
	store.DB.MustExec(`INSERT INTO track (id, fingerprint, url, duration) VALUES (1, "fp1", "http://foo", 123)`)

	// Insert predicates
	store.DB.MustExec(`INSERT INTO predicate (id) VALUES 
		("title"), ("composer"), ("producer"), ("language"), ("offence"),
		("artist"), ("mbid_artist")
	`)

	// Insert tags
	store.DB.MustExec(`INSERT INTO tag (trackid, predicateid, value) VALUES
		(1, "title", "My Song"),
		(1, "composer", "Alice, Bob"),
		(1, "producer", "Charlie"),
		(1, "language", "en, fr"),
		(1, "offence", "violence, bad words"),
		(1, "artist", "Eve"),
		(1, "mbid_artist", "1234-5678")
	`)

	req := httptest.NewRequest("GET", "/export.rdf", nil)
	w := httptest.NewRecorder()

	store.RDFHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	turtle := string(body)

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	// URL as resource
	if !strings.Contains(turtle, "<http://foo>") {
		t.Errorf("expected track URL as resource, got:\n%s", turtle)
	}

	// Duration
	if !strings.Contains(turtle, "PT123S") {
		t.Errorf("expected duration literal PT123S, got:\n%s", turtle)
	}

	// Title as literal
	if !strings.Contains(turtle, `"My Song"`) {
		t.Errorf("expected title literal, got:\n%s", turtle)
	}

	// Composer split into Alice and Bob (resources)
	if !strings.Contains(turtle, `<https://media-metadata.l42.eu/search?p.composer=Alice>`) || !strings.Contains(turtle, `<https://media-metadata.l42.eu/search?p.composer=Bob>`) {
		t.Errorf("expected composer literals Alice and Bob, got:\n%s", turtle)
	}

	// Producer
	if !strings.Contains(turtle, `<https://media-metadata.l42.eu/search?p.producer=Charlie>`) {
		t.Errorf("expected producer literal Charlie, got:\n%s", turtle)
	}

	// Language split into en and fr as resources
	if !strings.Contains(turtle, "<http://lexvo.org/id/iso639-3/en>") ||
		!strings.Contains(turtle, "<http://lexvo.org/id/iso639-3/fr>") {
		t.Errorf("expected language resources for en and fr, got:\n%s", turtle)
	}

	// Offence split into resources
	if !strings.Contains(turtle, `<https://media-metadata.l42.eu/search?p.offence=violence>`) || !strings.Contains(turtle, `<https://media-metadata.l42.eu/search?p.offence=bad+words>`) {
		t.Errorf("expected offence literals, got:\n%s", turtle)
	}

	// Artist as resource
	if !strings.Contains(turtle, `<https://media-metadata.l42.eu/search?p.artist=Eve>`) {
		t.Errorf("expected artist literal, got:\n%s", turtle)
	}

	// MBID artist as resource
	if !strings.Contains(turtle, "<https://musicbrainz.org/artist/1234-5678>") {
		t.Errorf("expected MBID artist as resource, got:\n%s", turtle)
	}
}
