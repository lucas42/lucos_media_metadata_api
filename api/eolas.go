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

// fetchEolasNames fetches human-readable names (skos:prefLabel) for the given
// URIs from the lucos_eolas bulk data endpoint. Returns a map of URI → name.
// Returns nil if KEY_LUCOS_EOLAS is not set or if eolas is unreachable.
func fetchEolasNames(uris []string) map[string]string {
	key := os.Getenv("KEY_LUCOS_EOLAS")
	if key == "" {
		slog.Warn("KEY_LUCOS_EOLAS not set, skipping name import from eolas")
		return nil
	}
	if len(uris) == 0 {
		return nil
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
		slog.Warn("Failed to create eolas request", slog.Any("error", err))
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "text/turtle")
	req.Header.Set("User-Agent", os.Getenv("SYSTEM"))

	slog.Info("Fetching entity names from eolas", "url", dataURL, "uri_count", len(uris))
	resp, err := client.Do(req)
	if err != nil {
		slog.Warn("Failed to fetch eolas data", slog.Any("error", err))
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		slog.Warn("Eolas returned non-200 status", "status", resp.StatusCode, "body", string(body))
		return nil
	}

	g := rdf2go.NewGraph("")
	if err := g.Parse(resp.Body, "text/turtle"); err != nil {
		slog.Warn("Failed to parse eolas RDF data", slog.Any("error", err))
		return nil
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
	return names
}

// fetchEolasName fetches the canonical name for a single eolas entity URI
// by looking it up in the bulk data endpoint. Returns an error if the URI
// is not found or if the fetch itself fails.
func fetchEolasName(uri string) (string, error) {
	names := fetchEolasNames([]string{uri})
	if names == nil {
		return "", fmt.Errorf("eolas fetch failed for %q", uri)
	}
	name, ok := names[uri]
	if !ok {
		return "", fmt.Errorf("no name found for %q in eolas data", uri)
	}
	return name, nil
}

// eolasLanguageURI builds the canonical eolas URI for a language code.
func eolasLanguageURI(code string) string {
	return fmt.Sprintf("%s/metadata/language/%s/", eolasOrigin, url.PathEscape(code))
}

// eolasPersonEntity is the JSON shape returned by the eolas Person list and create endpoints.
type eolasPersonEntity struct {
	ID   int    `json:"id"`
	URI  string `json:"uri"`
	Name string `json:"name"`
}

// resolveOrCreateEolasPersonHTTP looks up an existing eolas Person entity by
// name (case-insensitive). If no entity is found, it creates one via the eolas
// write API. Returns the entity URI.
//
// This is the production implementation of predicateconfig.EolasPersonResolver.
// Tests replace the package-level variable with a lightweight stub.
func resolveOrCreateEolasPersonHTTP(name string) (string, error) {
	key := os.Getenv("KEY_LUCOS_EOLAS")
	if key == "" {
		return "", fmt.Errorf("KEY_LUCOS_EOLAS not set; cannot resolve eolas Person for %q", name)
	}
	if eolasOrigin == "" {
		return "", fmt.Errorf("EOLAS_ORIGIN not set; cannot resolve eolas Person for %q", name)
	}

	client := &http.Client{Timeout: 30 * time.Second}

	// Fetch existing Person entities.
	listURL := eolasOrigin + "/metadata/person/list/"
	req, err := http.NewRequest("GET", listURL, nil)
	if err != nil {
		return "", fmt.Errorf("building person list request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", os.Getenv("SYSTEM"))

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching person list from eolas: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading person list response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("eolas person list returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var persons []eolasPersonEntity
	if err := json.Unmarshal(body, &persons); err != nil {
		return "", fmt.Errorf("parsing person list: %w", err)
	}

	// Case-insensitive name lookup.
	nameLower := strings.ToLower(name)
	for _, p := range persons {
		if strings.ToLower(p.Name) == nameLower {
			return p.URI, nil
		}
	}

	// Not found — create a new Person entity.
	createURL := eolasOrigin + "/api/metadata/person/"
	payload, err := json.Marshal(map[string]interface{}{"name": name})
	if err != nil {
		return "", fmt.Errorf("building create-person payload: %w", err)
	}
	createReq, err := http.NewRequest("POST", createURL, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("building create-person request: %w", err)
	}
	createReq.Header.Set("Authorization", "Bearer "+key)
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("User-Agent", os.Getenv("SYSTEM"))

	createResp, err := client.Do(createReq)
	if err != nil {
		return "", fmt.Errorf("creating eolas Person %q: %w", name, err)
	}
	defer createResp.Body.Close()

	createBody, err := io.ReadAll(createResp.Body)
	if err != nil {
		return "", fmt.Errorf("reading create-person response: %w", err)
	}

	// Check status before attempting to unmarshal — non-JSON bodies (proxied 503,
	// plain-text error pages) would otherwise produce a misleading parse error.
	switch createResp.StatusCode {
	case http.StatusCreated, http.StatusConflict: // 409 = already_exists (race between list and create)
		var created eolasPersonEntity
		if err := json.Unmarshal(createBody, &created); err != nil {
			return "", fmt.Errorf("parsing create-person response (HTTP %d): %w (body: %s)", createResp.StatusCode, err, string(createBody))
		}
		if created.URI == "" {
			return "", fmt.Errorf("eolas returned HTTP %d for Person %q but response had no URI: %s", createResp.StatusCode, name, string(createBody))
		}
		if createResp.StatusCode == http.StatusCreated {
			slog.Info("Created eolas Person entity", "name", name, "uri", created.URI)
		}
		return created.URI, nil
	default:
		return "", fmt.Errorf("eolas returned HTTP %d creating Person %q: %s", createResp.StatusCode, name, string(createBody))
	}
}
