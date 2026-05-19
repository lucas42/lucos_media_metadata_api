package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"testing"
)

// entityURI is a stable test URI that represents a deleted source entity.
const entityURI = "https://eolas.l42.eu/metadata/person/deleted-entity/"

// createTrackWithURITag creates a track via the v3 API with a language tag that
// has the given URI, and returns the track URL used.
func createTrackWithURITag(t *testing.T, fingerprint string, entityUri string) string {
	t.Helper()
	trackURL := "http://example.org/webhook-test/" + fingerprint
	escapedURL := url.QueryEscape(trackURL)
	path := fmt.Sprintf("/v3/tracks?url=%s", escapedURL)
	body := fmt.Sprintf(
		`{"fingerprint": %q, "duration": 100, "tags": {"language": [{"name": "English", "uri": %q}]}}`,
		fingerprint, entityUri,
	)
	req := basicRequest(t, "PUT", path, body)
	resp, _ := doRawRequest(t, req)
	if resp.StatusCode != 200 {
		t.Fatalf("Failed to create track: %d", resp.StatusCode)
	}
	return trackURL
}

// getTagsForTrack fetches the v3 tags for a track URL and returns them as a
// map[predicate][]TagValueV3.
func getTagsForTrack(t *testing.T, trackURL string) map[string][]TagValueV3 {
	t.Helper()
	escapedURL := url.QueryEscape(trackURL)
	path := fmt.Sprintf("/v3/tracks?url=%s", escapedURL)
	req := basicRequest(t, "GET", path, "")
	resp, err := doRawRequest(t, req)
	if err != nil || resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to GET track: status=%d body=%s", resp.StatusCode, string(body))
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var result struct {
		Tags map[string][]TagValueV3 `json:"tags"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("Failed to decode track response: %v", err)
	}
	return result.Tags
}

// postLoganneEvent posts a Loganne webhook event to /webhooks and returns the status code.
func postLoganneEvent(t *testing.T, eventType string, eventURL string) int {
	t.Helper()
	body := fmt.Sprintf(`{"type": %q, "url": %q, "source": "test"}`, eventType, eventURL)
	req := basicRequest(t, "POST", "/webhooks", body)
	resp, err := doRawRequest(t, req)
	if err != nil {
		t.Fatalf("Failed to POST to /webhooks: %v", err)
	}
	return resp.StatusCode
}

// ── Test 1: itemDeleted clears matching URI, name preserved ───────────────────

func TestItemDeletedClearsTagURI(t *testing.T) {
	clearData()
	trackURL := createTrackWithURITag(t, "wh-itemdeleted-1", entityURI)

	// Verify the URI is present before deletion
	tagsBefore := getTagsForTrack(t, trackURL)
	if len(tagsBefore["language"]) == 0 || tagsBefore["language"][0].URI != entityURI {
		t.Fatalf("Expected language tag with URI=%q before event, got: %+v", entityURI, tagsBefore["language"])
	}
	nameBefore := tagsBefore["language"][0].Name

	// Post itemDeleted event
	status := postLoganneEvent(t, "itemDeleted", entityURI)
	assertEqual(t, "itemDeleted: expected 204", 204, status)

	// Verify URI is cleared; name is preserved
	tagsAfter := getTagsForTrack(t, trackURL)
	if len(tagsAfter["language"]) == 0 {
		t.Fatal("language tag disappeared entirely; expected it to remain with name but no URI")
	}
	tagAfter := tagsAfter["language"][0]
	if tagAfter.URI != "" {
		t.Errorf("itemDeleted: expected URI to be cleared, got %q", tagAfter.URI)
	}
	if tagAfter.Name != nameBefore {
		t.Errorf("itemDeleted: name changed; before=%q after=%q", nameBefore, tagAfter.Name)
	}
}

// ── Test 3: idempotent re-delivery ───────────────────────────────────────────

func TestItemDeletedIsIdempotent(t *testing.T) {
	clearData()
	trackURL := createTrackWithURITag(t, "wh-idempotent-1", entityURI)

	// First delivery
	status1 := postLoganneEvent(t, "itemDeleted", entityURI)
	assertEqual(t, "idempotent: first delivery expected 204", 204, status1)

	// Second delivery of the same event — must not error
	status2 := postLoganneEvent(t, "itemDeleted", entityURI)
	assertEqual(t, "idempotent: second delivery expected 204", 204, status2)

	// State after double delivery is the same as after single delivery
	tagsAfter := getTagsForTrack(t, trackURL)
	if len(tagsAfter["language"]) == 0 {
		t.Fatal("language tag disappeared on idempotent re-delivery")
	}
	if tagsAfter["language"][0].URI != "" {
		t.Errorf("idempotent: URI not cleared after second delivery, got %q", tagsAfter["language"][0].URI)
	}
}

// ── Test 4: no matching rows — handler returns without error ──────────────────

func TestItemDeletedNoMatchingRows(t *testing.T) {
	clearData()
	// Post deletion for a URI that no tag references
	status := postLoganneEvent(t, "itemDeleted", "https://eolas.l42.eu/metadata/person/nonexistent/")
	assertEqual(t, "no-match: expected 204", 204, status)
}

// ── Test 5: unrecognised event type is silently ignored ───────────────────────

func TestUnknownLoganneEventIgnored(t *testing.T) {
	clearData()
	trackURL := createTrackWithURITag(t, "wh-unknown-event-1", entityURI)

	// A genuinely unrecognised event type must not touch any tags
	status := postLoganneEvent(t, "someOtherEvent", entityURI)
	assertEqual(t, "unrecognised event: expected 204", 204, status)

	// URI must still be present
	tags := getTagsForTrack(t, trackURL)
	if len(tags["language"]) == 0 || tags["language"][0].URI != entityURI {
		t.Errorf("unrecognised event: URI should be unchanged, got: %+v", tags["language"])
	}
}

// ── Test 6: GET to /webhooks is rejected ──────────────────────────────────────

func TestWebhooksEndpointRejectsGet(t *testing.T) {
	clearData()
	makeRequestWithUnallowedMethod(t, "/webhooks", "GET", []string{"POST"})
}

// ── itemUpdated tests ────────────────────────────────────────────────────────

// withMockNameFetcher installs a mock entityNameFetcher for the duration of a
// test and restores the original on cleanup.
func withMockNameFetcher(t *testing.T, names map[string]string) {
	t.Helper()
	orig := entityNameFetcher
	entityNameFetcher = func(uri string) (string, error) {
		if name, ok := names[uri]; ok {
			return name, nil
		}
		return "", fmt.Errorf("no mock name for %q", uri)
	}
	t.Cleanup(func() { entityNameFetcher = orig })
}

// ── Test 7: itemUpdated refreshes the stored name ────────────────────────────

func TestItemUpdatedRefreshesTagName(t *testing.T) {
	clearData()
	withMockNameFetcher(t, map[string]string{entityURI: "Updated Entity Name"})

	trackURL := createTrackWithURITag(t, "wh-item-updated-1", entityURI)

	// Confirm original name is stored
	tagsBefore := getTagsForTrack(t, trackURL)
	if len(tagsBefore["language"]) == 0 {
		t.Fatal("expected language tag before update")
	}
	if tagsBefore["language"][0].URI != entityURI {
		t.Fatalf("expected URI=%q, got %q", entityURI, tagsBefore["language"][0].URI)
	}
	// The mock returns "Updated Entity Name" — after the event the stored name should change.

	status := postLoganneEvent(t, "itemUpdated", entityURI)
	assertEqual(t, "itemUpdated: expected 204", 204, status)

	tagsAfter := getTagsForTrack(t, trackURL)
	if len(tagsAfter["language"]) == 0 {
		t.Fatal("language tag disappeared after itemUpdated")
	}
	tagAfter := tagsAfter["language"][0]
	if tagAfter.URI != entityURI {
		t.Errorf("itemUpdated: URI should be unchanged, got %q", tagAfter.URI)
	}
	assertEqual(t, "itemUpdated: name should be refreshed", "Updated Entity Name", tagAfter.Name)
}

// ── Test 9: itemUpdated with no matching tags is a no-op ─────────────────────

func TestItemUpdatedNoMatchingTagsIsNoop(t *testing.T) {
	clearData()
	withMockNameFetcher(t, map[string]string{entityURI: "Some Name"})

	// Don't create any track/tag with entityURI — just post the event.
	status := postLoganneEvent(t, "itemUpdated", "https://eolas.l42.eu/metadata/person/nonexistent/")
	// The mock has no entry for this URI so the fetcher returns an error — handler
	// swallows it (best-effort) and still returns 204.
	assertEqual(t, "no-match itemUpdated: expected 204", 204, status)
}

// ── Test 10: itemUpdated is idempotent ────────────────────────────────────────

func TestItemUpdatedIsIdempotent(t *testing.T) {
	clearData()
	withMockNameFetcher(t, map[string]string{entityURI: "Stable Name"})

	trackURL := createTrackWithURITag(t, "wh-item-updated-idempotent-1", entityURI)

	status1 := postLoganneEvent(t, "itemUpdated", entityURI)
	assertEqual(t, "idempotent first delivery: expected 204", 204, status1)

	status2 := postLoganneEvent(t, "itemUpdated", entityURI)
	assertEqual(t, "idempotent second delivery: expected 204", 204, status2)

	tagsAfter := getTagsForTrack(t, trackURL)
	if len(tagsAfter["language"]) == 0 {
		t.Fatal("language tag disappeared after idempotent re-delivery")
	}
	assertEqual(t, "idempotent: name should remain stable", "Stable Name", tagsAfter["language"][0].Name)
}
