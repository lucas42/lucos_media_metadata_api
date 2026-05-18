package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"
)

// fetchContactName fetches the canonical name for a single contact by its URI.
// Sends a GET request to the URI with Accept: application/json and the
// KEY_LUCOS_CONTACTS Bearer token. Returns the "name" field from the JSON response.
func fetchContactName(uri string) (string, error) {
	key := os.Getenv("KEY_LUCOS_CONTACTS")
	if key == "" {
		return "", fmt.Errorf("KEY_LUCOS_CONTACTS not set")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return "", fmt.Errorf("failed to build contacts request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("User-Agent", os.Getenv("SYSTEM"))

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("contacts request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return "", fmt.Errorf("contacts returned %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode contacts response: %w", err)
	}
	if result.Name == "" {
		slog.Warn("fetchContactName: response had empty name field", "uri", uri)
	}
	return result.Name, nil
}

// fetchContactNames fetches canonical names for a slice of contact URIs,
// making one HTTP request per URI. Returns a URI→name map; URIs where the
// fetch failed are omitted from the map (errors are logged, not propagated).
func fetchContactNames(uris []string) map[string]string {
	if len(uris) == 0 {
		return nil
	}
	names := make(map[string]string, len(uris))
	for _, uri := range uris {
		name, err := fetchContactName(uri)
		if err != nil {
			slog.Warn("fetchContactNames: failed to fetch contact", "uri", uri, slog.Any("error", err))
			continue
		}
		names[uri] = name
	}
	if len(names) == 0 {
		return nil
	}
	return names
}
