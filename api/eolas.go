package main

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"

	"github.com/deiu/rdf2go"
)

const eolasOrigin = "https://eolas.l42.eu"
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
	eolasHost, _ := url.Parse(eolasOrigin)

	// Only reattach auth header when the redirect stays on the same host
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Host == eolasHost.Host {
				req.Header.Set("Authorization", "key "+key)
			}
			return nil
		},
	}

	req, err := http.NewRequest("GET", dataURL, nil)
	if err != nil {
		slog.Warn("Failed to create eolas request", slog.Any("error", err))
		return nil
	}
	req.Header.Set("Authorization", "key "+key)
	req.Header.Set("Accept", "text/turtle")
	req.Header.Set("User-Agent", "lucos_media_metadata_api")

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

// eolasLanguageURI builds the canonical eolas URI for a language code.
func eolasLanguageURI(code string) string {
	return fmt.Sprintf("%s/metadata/language/%s/", eolasOrigin, url.PathEscape(code))
}
