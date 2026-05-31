package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/deiu/rdf2go"
)

// eolasOrigin is the base URL of the lucos_eolas service, read from the
// EOLAS_ORIGIN environment variable. It is used both for fetching data and as
// the allowed origin for URI validation of eolas predicates.
var eolasOrigin = os.Getenv("EOLAS_ORIGIN")

const eolasDataPath = "/metadata/all/data/"
const prefLabelURI = "http://www.w3.org/2004/02/skos/core#prefLabel"

// eolasClientTimeout is the HTTP timeout for eolas calls on the request
// (save) path. Short by design — a bounded fast failure beats a 30-second
// hang that cascades into a gateway 502. Background jobs (e.g. reconcile)
// use their own, longer timeout since latency there does not block a user.
const eolasClientTimeout = 3 * time.Second

// fetchEolasNames fetches human-readable names (skos:prefLabel) for the given
// URIs from the lucos_eolas bulk data endpoint. Returns a map of URI → name.
//
// A non-nil error means the names could not be retrieved: missing credentials
// (KEY_LUCOS_EOLAS unset), transport error / timeout, non-200 response, or
// unparseable RDF. The daily reconcile job treats this as a job failure rather
// than silently reporting success on a no-op run (see #303). A successful fetch
// that happens to resolve zero names is NOT an error: the trigger is the failed
// retrieval, not the resolved-count.
//
// Returns (nil, nil) — a non-error skip — only when there is genuinely nothing
// to do: no URIs to resolve.
func fetchEolasNames(uris []string) (map[string]string, error) {
	key := os.Getenv("KEY_LUCOS_EOLAS")
	if key == "" {
		return nil, fmt.Errorf("KEY_LUCOS_EOLAS not set; cannot fetch names from eolas")
	}
	if len(uris) == 0 {
		return nil, nil
	}

	// Build a set of URIs we're interested in for fast lookup
	wanted := make(map[string]bool, len(uris))
	for _, uri := range uris {
		wanted[uri] = true
	}

	dataURL := eolasOrigin + eolasDataPath
	eolasBaseURL, _ := url.Parse(eolasOrigin)

	// Only reattach auth header when the redirect stays on the same host
	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Host == eolasBaseURL.Host {
				req.Header.Set("Authorization", "Bearer "+key)
			}
			return nil
		},
	}

	req, err := http.NewRequest("GET", dataURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building eolas bulk request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "text/turtle")
	req.Header.Set("User-Agent", os.Getenv("SYSTEM"))

	slog.Info("Fetching entity names from eolas", "url", dataURL, "uri_count", len(uris))
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching eolas data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("eolas returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	g := rdf2go.NewGraph("")
	if err := g.Parse(resp.Body, "text/turtle"); err != nil {
		return nil, fmt.Errorf("parsing eolas RDF data: %w", err)
	}

	prefLabel := rdf2go.NewResource(prefLabelURI)
	names := make(map[string]string, len(uris))
	for triple := range g.IterTriples() {
		if !triple.Predicate.Equal(prefLabel) {
			continue
		}
		subjectURI := triple.Subject.RawValue()
		if !wanted[subjectURI] {
			continue
		}
		names[subjectURI] = triple.Object.RawValue()
	}

	slog.Info("Fetched entity names from eolas", "requested", len(uris), "resolved", len(names))
	return names, nil
}

// fetchEolasName fetches the canonical name (skos:prefLabel) for a single
// eolas entity using the per-entity data endpoint (/metadata/{type}/{pk}/data/).
// This is significantly cheaper than the bulk /metadata/all/data/ endpoint —
// it fetches only the RDF graph for the requested entity, not the whole dataset.
// Uses eolasClientTimeout so it fails fast if eolas is slow or unavailable.
func fetchEolasName(uri string) (string, error) {
	// Belt-and-braces origin guard: URI must start with the configured eolas
	// origin. This makes the function self-protecting regardless of how callers
	// evolve, and removes any CodeQL SSRF concern at the point of URL construction.
	if eolasOrigin == "" {
		return "", fmt.Errorf("EOLAS_ORIGIN not set; cannot fetch name for %q", uri)
	}
	if !strings.HasPrefix(uri, eolasOrigin+"/") {
		return "", fmt.Errorf("fetchEolasName: URI %q does not start with configured eolas origin", uri)
	}

	key := os.Getenv("KEY_LUCOS_EOLAS")
	if key == "" {
		return "", fmt.Errorf("KEY_LUCOS_EOLAS not set; cannot resolve name for %q", uri)
	}

	// Construct the per-entity data URL: append "data/" after the entity path.
	// Eolas entity URIs consistently end with a trailing slash.
	dataURL := strings.TrimRight(uri, "/") + "/data/"

	eolasBaseURL, _ := url.Parse(eolasOrigin)
	client := &http.Client{
		Timeout: eolasClientTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Host == eolasBaseURL.Host {
				req.Header.Set("Authorization", "Bearer "+key)
			}
			return nil
		},
	}

	req, err := http.NewRequest("GET", dataURL, nil)
	if err != nil {
		return "", fmt.Errorf("building eolas entity request for %q: %w", uri, err)
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "text/turtle")
	req.Header.Set("User-Agent", os.Getenv("SYSTEM"))

	slog.Debug("Fetching entity name from eolas", "uri", uri, "url", dataURL)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching eolas entity %q: %w", uri, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("eolas returned HTTP %d for entity %q: %s", resp.StatusCode, uri, string(body))
	}

	// Parse the entity-scoped Turtle graph. Use the entity URI as the base so
	// that any relative references in the Turtle resolve correctly.
	g := rdf2go.NewGraph(uri)
	if err := g.Parse(resp.Body, "text/turtle"); err != nil {
		return "", fmt.Errorf("parsing eolas entity RDF for %q: %w", uri, err)
	}

	prefLabel := rdf2go.NewResource(prefLabelURI)
	subject := rdf2go.NewResource(uri)
	for triple := range g.IterTriples() {
		if triple.Predicate.Equal(prefLabel) && triple.Subject.Equal(subject) {
			return triple.Object.RawValue(), nil
		}
	}

	return "", fmt.Errorf("no prefLabel found for %q in eolas entity data", uri)
}

// eolasLanguageURI builds the canonical eolas URI for a language code.
func eolasLanguageURI(code string) string {
	return fmt.Sprintf("%s/metadata/language/%s/", eolasOrigin, url.PathEscape(code))
}

// eolasEntityResponse is the JSON shape returned by the eolas entity create endpoint.
type eolasEntityResponse struct {
	ID   int    `json:"id"`
	URI  string `json:"uri"`
	Name string `json:"name"`
}

// resolveOrCreateEolasEntityFn is the function used to resolve-or-create eolas
// entities at tag-write time. Overridable in tests via a package-level swap.
var resolveOrCreateEolasEntityFn = resolveOrCreateEolasEntityHTTP

// resolveOrCreateEolasEntityHTTP POSTs a new entity of the given type to eolas.
// On 201 (created) or 409 (already_exists) it returns the entity URI.
// The 409 case naturally covers concurrent creation — eolas returns the existing
// entity so no pre-listing is needed.
//
// entityType must match an eolas model slug (e.g. "person").
func resolveOrCreateEolasEntityHTTP(entityType, name string) (string, error) {
	key := os.Getenv("KEY_LUCOS_EOLAS")
	if key == "" {
		return "", fmt.Errorf("KEY_LUCOS_EOLAS not set; cannot resolve eolas %s for %q", entityType, name)
	}
	if eolasOrigin == "" {
		return "", fmt.Errorf("EOLAS_ORIGIN not set; cannot resolve eolas %s for %q", entityType, name)
	}

	createURL := eolasOrigin + "/api/metadata/" + url.PathEscape(entityType) + "/"
	payload, err := json.Marshal(map[string]interface{}{"name": name})
	if err != nil {
		return "", fmt.Errorf("building create-%s payload: %w", entityType, err)
	}
	req, err := http.NewRequest("POST", createURL, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("building create-%s request: %w", entityType, err)
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", os.Getenv("SYSTEM"))

	client := &http.Client{Timeout: eolasClientTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("creating eolas %s %q: %w", entityType, name, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading create-%s response: %w", entityType, err)
	}

	// Check status before unmarshalling — non-JSON error bodies (proxied 503,
	// plain-text 500) would otherwise produce a misleading parse failure.
	switch resp.StatusCode {
	case http.StatusCreated, http.StatusConflict: // 409 = already_exists
		var entity eolasEntityResponse
		if err := json.Unmarshal(body, &entity); err != nil {
			return "", fmt.Errorf("parsing create-%s response (HTTP %d): %w (body: %s)", entityType, resp.StatusCode, err, string(body))
		}
		if entity.URI == "" {
			return "", fmt.Errorf("eolas returned HTTP %d for %s %q but response had no URI: %s", resp.StatusCode, entityType, name, string(body))
		}
		if resp.StatusCode == http.StatusCreated {
			slog.Info("Created eolas entity", "type", entityType, "name", name, "uri", entity.URI)
		}
		return entity.URI, nil
	default:
		return "", fmt.Errorf("eolas returned HTTP %d creating %s %q: %s", resp.StatusCode, entityType, name, string(body))
	}
}

// ResolveOrCreateEolasEntityByName satisfies the predicateconfig.NameURIResolver
// interface. It creates (or retrieves) an eolas entity of the given type by name
// and returns its URI. Used by the composer and producer predicates.
func (store Datastore) ResolveOrCreateEolasEntityByName(entityType, name string) (string, error) {
	return resolveOrCreateEolasEntityFn(entityType, name)
}

// ResolveEolasEntityName satisfies the predicateconfig.NameURIResolver interface.
// It returns the canonical name for a given eolas entity URI. Used by the composer
// and producer predicates when a tag is written with a URI but no name.
func (store Datastore) ResolveEolasEntityName(uri string) (string, error) {
	return entityNameFetcher(uri)
}
