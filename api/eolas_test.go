package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ── fetchEolasNames HTTP-level tests ─────────────────────────────────────────
// These tests spin up a minimal httptest.Server that stands in for eolas and
// verify that fetchEolasNames sends the right request and correctly handles
// various responses.  The reconcile-layer tests (reconcile_test.go) mock the
// fetcher function directly and don't test the HTTP wire protocol.

// newMockEolasServer creates a test server that handles POST /metadata/names.
// The handler function receives the decoded URI list and returns the names map
// that the mock should respond with (HTTP 200 + JSON).  Pass a nil handler to
// get a server that always responds 500.
func newMockEolasServer(t *testing.T, handler func(uris []string) map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handler == nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/metadata/names" {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read error", http.StatusInternalServerError)
			return
		}
		var uris []string
		if err := json.Unmarshal(body, &uris); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		names := handler(uris)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(names)
	}))
}

// TestFetchEolasNamesHappyPath verifies that fetchEolasNames returns the names
// map from a successful POST /metadata/names response.
func TestFetchEolasNamesHappyPath(t *testing.T) {
	t.Setenv("KEY_LUCOS_EOLAS", "testkey")

	srv := newMockEolasServer(t, func(uris []string) map[string]string {
		// Echo back each URI with a synthesised name so we can assert both
		// that all sent URIs were resolved and that the values came back correctly.
		result := make(map[string]string, len(uris))
		for _, u := range uris {
			result[u] = "Name for " + u
		}
		return result
	})
	defer srv.Close()

	origOrigin := eolasOrigin
	eolasOrigin = srv.URL
	defer func() { eolasOrigin = origOrigin }()

	uris := []string{
		srv.URL + "/metadata/person/1/",
		srv.URL + "/metadata/person/2/",
	}
	names, err := fetchEolasNames(uris)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("expected 2 resolved names, got %d: %v", len(names), names)
	}
	for _, uri := range uris {
		expected := "Name for " + uri
		if names[uri] != expected {
			t.Errorf("for %s: expected %q, got %q", uri, expected, names[uri])
		}
	}
}

// TestFetchEolasNamesSendsCorrectRequest verifies the request shape:
// POST to /metadata/names with Authorization header and JSON body.
func TestFetchEolasNamesSendsCorrectRequest(t *testing.T) {
	t.Setenv("KEY_LUCOS_EOLAS", "secret-key")

	var capturedMethod, capturedPath, capturedAuth, capturedCT string
	var capturedBody []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		capturedAuth = r.Header.Get("Authorization")
		capturedCT = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{})
	}))
	defer srv.Close()

	origOrigin := eolasOrigin
	eolasOrigin = srv.URL
	defer func() { eolasOrigin = origOrigin }()

	uri := srv.URL + "/metadata/person/42/"
	fetchEolasNames([]string{uri})

	if capturedMethod != "POST" {
		t.Errorf("expected POST, got %s", capturedMethod)
	}
	if capturedPath != "/metadata/names" {
		t.Errorf("expected /metadata/names, got %s", capturedPath)
	}
	if capturedAuth != "Bearer secret-key" {
		t.Errorf("expected Bearer secret-key, got %q", capturedAuth)
	}
	if capturedCT != "application/json" {
		t.Errorf("expected application/json Content-Type, got %q", capturedCT)
	}
	if len(capturedBody) != 1 || capturedBody[0] != uri {
		t.Errorf("expected body [%q], got %v", uri, capturedBody)
	}
}

// TestFetchEolasNamesPartialResolution verifies that URIs not found in eolas
// (omitted from the response map) are simply absent from the result — no error.
func TestFetchEolasNamesPartialResolution(t *testing.T) {
	t.Setenv("KEY_LUCOS_EOLAS", "testkey")

	srv := newMockEolasServer(t, func(uris []string) map[string]string {
		// Only resolve the first URI; omit the second.
		if len(uris) > 0 {
			return map[string]string{uris[0]: "Known Name"}
		}
		return map[string]string{}
	})
	defer srv.Close()

	origOrigin := eolasOrigin
	eolasOrigin = srv.URL
	defer func() { eolasOrigin = origOrigin }()

	known := srv.URL + "/metadata/person/1/"
	unknown := srv.URL + "/metadata/person/99999/"
	names, err := fetchEolasNames([]string{known, unknown})
	if err != nil {
		t.Fatalf("expected no error for partial resolution, got: %v", err)
	}
	if names[known] != "Known Name" {
		t.Errorf("expected 'Known Name' for %s, got %q", known, names[known])
	}
	if _, ok := names[unknown]; ok {
		t.Errorf("expected unknown URI to be absent from result, but it was present")
	}
}

// TestFetchEolasNamesNonOKResponseErrors verifies that a non-200 response from
// eolas is surfaced as an error (not a silent no-op).
func TestFetchEolasNamesNonOKResponseErrors(t *testing.T) {
	t.Setenv("KEY_LUCOS_EOLAS", "testkey")

	srv := newMockEolasServer(t, nil) // nil → always 500
	defer srv.Close()

	origOrigin := eolasOrigin
	eolasOrigin = srv.URL
	defer func() { eolasOrigin = origOrigin }()

	_, err := fetchEolasNames([]string{srv.URL + "/metadata/person/1/"})
	if err == nil {
		t.Fatal("expected an error for a non-200 response, got nil")
	}
}

// TestFetchEolasNamesEmptyListReturnsNil verifies that an empty URI list
// returns (nil, nil) without making any HTTP request.
func TestFetchEolasNamesEmptyListReturnsNil(t *testing.T) {
	t.Setenv("KEY_LUCOS_EOLAS", "testkey")

	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
	}))
	defer srv.Close()

	origOrigin := eolasOrigin
	eolasOrigin = srv.URL
	defer func() { eolasOrigin = origOrigin }()

	names, err := fetchEolasNames([]string{})
	if err != nil {
		t.Fatalf("expected no error for empty list, got: %v", err)
	}
	if names != nil {
		t.Errorf("expected nil names for empty list, got: %v", names)
	}
	if requestCount != 0 {
		t.Errorf("expected no HTTP requests for empty list, got %d", requestCount)
	}
}
