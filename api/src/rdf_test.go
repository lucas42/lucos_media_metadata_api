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

	if ct := rr.Header().Get("Content-Type"); ct != "text/turtle" {
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
