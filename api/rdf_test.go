package main

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRDFHandler_ServesFile(t *testing.T) {
	tempDir := t.TempDir()
	rdfFile := filepath.Join(tempDir, "test.ttl")
	content := "@prefix dc: <http://purl.org/dc/terms/> .\n<http://example.org/track/1> dc:title \"Song A\" ."
	if err := ioutil.WriteFile(rdfFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp RDF file: %v", err)
	}

	os.Setenv("RDF_OUTPUT_PATH", rdfFile)
	defer os.Unsetenv("RDF_OUTPUT_PATH")

	req := httptest.NewRequest(http.MethodGet, "/rdf", nil)
	rr := httptest.NewRecorder()

	RDFHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/turtle") {
		t.Errorf("expected Content-Type text/turtle, got %s", ct)
	}

	if !strings.Contains(rr.Body.String(), "Song A") {
		t.Errorf("response body missing RDF content: %s", rr.Body.String())
	}
}

func TestRDFHandler_NoEnvSet(t *testing.T) {
	os.Unsetenv("RDF_OUTPUT_PATH")
	req := httptest.NewRequest(http.MethodGet, "/rdf", nil)
	rr := httptest.NewRecorder()

	RDFHandler(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 when RDF_OUTPUT_PATH not set, got %d", rr.Code)
	}
}

func TestRDFHandler_FileNotFound(t *testing.T) {
	os.Setenv("RDF_OUTPUT_PATH", "/nonexistent/file.ttl")
	defer os.Unsetenv("RDF_OUTPUT_PATH")

	req := httptest.NewRequest(http.MethodGet, "/rdf", nil)
	rr := httptest.NewRecorder()

	RDFHandler(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing RDF file, got %d", rr.Code)
	}
}



func TestPrefersRDF(t *testing.T) {
	tests := []struct {
		name           string
		acceptHeader   string
		wantIsRDF      bool
		wantMime       string
	}{
		{
			name:         "No Accept header",
			acceptHeader: "",
			wantIsRDF:    false,
			wantMime:     "application/json",
		},
		{
			name:         "Prefers RDF",
			acceptHeader: "text/turtle,application/json",
			wantIsRDF:    true,
			wantMime:     "text/turtle",
		},
		{
			name:         "Prefers JSON over RDF",
			acceptHeader: "application/json,text/turtle",
			wantIsRDF:    false,
			wantMime:     "application/json",
		},
		{
			name:         "Other mimetypes, none JSON or RDF",
			acceptHeader: "text/html,application/xml",
			wantIsRDF:    false,
			wantMime:     "application/json", // fallback
		},
		{
			name:         "Ignores XML serialisation of RDF",
			acceptHeader: "application/rdf+xml,text/turtle",
			wantIsRDF:    true,
			wantMime:     "text/turtle",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/", nil)
			if tc.acceptHeader != "" {
				req.Header.Set("Accept", tc.acceptHeader)
			}

			gotIsRDF, gotMime := prefersRDF(req)
			if gotIsRDF != tc.wantIsRDF || gotMime != tc.wantMime {
				t.Errorf("prefersRDF(%q) = (%v, %q), want (%v, %q)",
					tc.acceptHeader, gotIsRDF, gotMime, tc.wantIsRDF, tc.wantMime)
			}
		})
	}
}

/**
 * Retrieves a single track output in RDF
 */
func TestRDFOutputForSingleTrack(test *testing.T) {
	clearData()
	trackpath := "/v2/tracks/1"
	inputJson := `{"fingerprint": "cdb567", "url": "http://example.org/track/879", "tags": {"title": "Rocky Diamond Forest"}, "duration": 300}`
	outputJson := `{"fingerprint": "cdb567", "duration": 300, "url": "http://example.org/track/879", "trackid": 1, "tags": {"title": "Rocky Diamond Forest"}, "collections":[], "weighting": 0}`
	makeRequest(test, "PUT", trackpath, inputJson, 200, outputJson, true)

	request := basicRequest(test, "GET", trackpath, "")
	request.Header.Set("Accept", "application/rdf+xml, text/turtle")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		test.Error(err)
	}
	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		test.Error(err)
	}
	actualResponseBody := string(responseData)

	if response.StatusCode >= 500 {
		test.Errorf("Error code %d returned for %s, message: %s", response.StatusCode, trackpath, actualResponseBody)
		return
	}
	if response.StatusCode != 200 {
		test.Errorf("Got non 200 response code %d for %s", response.StatusCode, trackpath)
	}

	if !strings.Contains(actualResponseBody, "<https://media-metadata.l42.eu/tracks/1>") {
		test.Errorf("Track URI not returned in RDF representation `%s`", actualResponseBody)
	}
	if !strings.Contains(actualResponseBody, "<http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <http://purl.org/ontology/mo/Track>") {
		test.Errorf("Track type not returned in RDF representation `%s`", actualResponseBody)
	}
	if !strings.Contains(actualResponseBody, "<http://purl.org/dc/terms/identifier> \"http://example.org/track/879\"") {
		test.Errorf("Track's mediaURL not returned in RDF representation `%s`", actualResponseBody)
	}
	if !strings.Contains(actualResponseBody, "<http://www.w3.org/2004/02/skos/core#prefLabel> \"Rocky Diamond Forest\"") {
		test.Errorf("Track's title not returned in RDF representation `%s`", actualResponseBody)
	}
}