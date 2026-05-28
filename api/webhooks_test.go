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

// postLoganneEventWithBody posts a raw JSON body to /webhooks and returns the status code.
func postLoganneEventWithBody(t *testing.T, body string) int {
	t.Helper()
	req := basicRequest(t, "POST", "/webhooks", body)
	resp, err := doRawRequest(t, req)
	if err != nil {
		t.Fatalf("Failed to POST to /webhooks: %v", err)
	}
	return resp.StatusCode
}

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

// ── contactLinked tests ──────────────────────────────────────────────────────

const contactURI = "https://eolas.l42.eu/metadata/person/contact-123/"
const eolasPersonURI = "https://eolas.l42.eu/metadata/person/alice/"
const previousEolasURI = "https://eolas.l42.eu/metadata/person/previous-eolas/"

// ── Test 11: contactLinked (initial link) rewrites contactUri → eolasUri and refreshes name

func TestContactLinkedInitialLinkRewritesURIAndName(t *testing.T) {
	clearData()
	withMockNameFetcher(t, map[string]string{eolasPersonURI: "Alice Smith"})

	trackURL := createTrackWithURITag(t, "wh-contact-linked-initial-1", contactURI)

	// Verify the contact URI is stored before the event
	tagsBefore := getTagsForTrack(t, trackURL)
	if len(tagsBefore["language"]) == 0 || tagsBefore["language"][0].URI != contactURI {
		t.Fatalf("expected language tag with contactURI=%q, got: %+v", contactURI, tagsBefore["language"])
	}

	// Initial link: previousEolasUri is null
	status := postLoganneEventWithBody(t, fmt.Sprintf(
		`{"type": "contactLinked", "contactUri": %q, "eolasUri": %q, "previousEolasUri": null}`,
		contactURI, eolasPersonURI,
	))
	assertEqual(t, "contactLinked initial: expected 204", 204, status)

	tagsAfter := getTagsForTrack(t, trackURL)
	if len(tagsAfter["language"]) == 0 {
		t.Fatal("language tag disappeared after contactLinked")
	}
	tagAfter := tagsAfter["language"][0]
	assertEqual(t, "contactLinked initial: URI should be rewritten to eolasUri", eolasPersonURI, tagAfter.URI)
	assertEqual(t, "contactLinked initial: name should be refreshed", "Alice Smith", tagAfter.Name)
}

// ── Test 12: contactLinked (relink) rewrites previousEolasUri → eolasUri and refreshes name

func TestContactLinkedRelinkRewritesPreviousEolasUriAndName(t *testing.T) {
	clearData()
	withMockNameFetcher(t, map[string]string{eolasPersonURI: "Alice Smith (Relinked)"})

	// Tag currently references the previous eolas URI (set by an earlier contactLinked event)
	trackURL := createTrackWithURITag(t, "wh-contact-linked-relink-1", previousEolasURI)

	tagsBefore := getTagsForTrack(t, trackURL)
	if len(tagsBefore["language"]) == 0 || tagsBefore["language"][0].URI != previousEolasURI {
		t.Fatalf("expected language tag with previousEolasURI=%q, got: %+v", previousEolasURI, tagsBefore["language"])
	}

	// Relink: previousEolasUri is the old eolas URI
	status := postLoganneEventWithBody(t, fmt.Sprintf(
		`{"type": "contactLinked", "contactUri": %q, "eolasUri": %q, "previousEolasUri": %q}`,
		contactURI, eolasPersonURI, previousEolasURI,
	))
	assertEqual(t, "contactLinked relink: expected 204", 204, status)

	tagsAfter := getTagsForTrack(t, trackURL)
	if len(tagsAfter["language"]) == 0 {
		t.Fatal("language tag disappeared after contactLinked relink")
	}
	tagAfter := tagsAfter["language"][0]
	assertEqual(t, "contactLinked relink: URI should be rewritten to new eolasUri", eolasPersonURI, tagAfter.URI)
	assertEqual(t, "contactLinked relink: name should be refreshed", "Alice Smith (Relinked)", tagAfter.Name)
}

// ── Test 13: contactLinked with no matching tags is a no-op ──────────────────

func TestContactLinkedNoMatchingTagsIsNoop(t *testing.T) {
	clearData()
	withMockNameFetcher(t, map[string]string{eolasPersonURI: "Alice Smith"})

	status := postLoganneEventWithBody(t, fmt.Sprintf(
		`{"type": "contactLinked", "contactUri": %q, "eolasUri": %q, "previousEolasUri": null}`,
		"https://eolas.l42.eu/metadata/person/nonexistent-contact/", eolasPersonURI,
	))
	assertEqual(t, "contactLinked no-match: expected 204", 204, status)
}

// ── Test 14: contactLinked is idempotent ─────────────────────────────────────

func TestContactLinkedIsIdempotent(t *testing.T) {
	clearData()
	withMockNameFetcher(t, map[string]string{eolasPersonURI: "Stable Person Name"})

	trackURL := createTrackWithURITag(t, "wh-contact-linked-idempotent-1", contactURI)

	payload := fmt.Sprintf(
		`{"type": "contactLinked", "contactUri": %q, "eolasUri": %q, "previousEolasUri": null}`,
		contactURI, eolasPersonURI,
	)
	status1 := postLoganneEventWithBody(t, payload)
	assertEqual(t, "contactLinked idempotent: first delivery expected 204", 204, status1)

	status2 := postLoganneEventWithBody(t, payload)
	assertEqual(t, "contactLinked idempotent: second delivery expected 204", 204, status2)

	tagsAfter := getTagsForTrack(t, trackURL)
	if len(tagsAfter["language"]) == 0 {
		t.Fatal("language tag disappeared after idempotent contactLinked")
	}
	tagAfter := tagsAfter["language"][0]
	assertEqual(t, "contactLinked idempotent: URI should be stable", eolasPersonURI, tagAfter.URI)
	assertEqual(t, "contactLinked idempotent: name should be stable", "Stable Person Name", tagAfter.Name)
}

// ── itemMerged tests ──────────────────────────────────────────────────────────

const sourceEntityURI = "https://eolas.l42.eu/metadata/person/source-entity/"
const targetEntityURI = "https://eolas.l42.eu/metadata/person/target-entity/"

// ── Test 15: itemMerged rewrites sourceUri → targetUri and refreshes name

func TestItemMergedRewritesURIAndName(t *testing.T) {
	clearData()
	withMockNameFetcher(t, map[string]string{targetEntityURI: "Target Entity Name"})

	trackURL := createTrackWithURITag(t, "wh-item-merged-1", sourceEntityURI)

	tagsBefore := getTagsForTrack(t, trackURL)
	if len(tagsBefore["language"]) == 0 || tagsBefore["language"][0].URI != sourceEntityURI {
		t.Fatalf("expected language tag with sourceEntityURI=%q, got: %+v", sourceEntityURI, tagsBefore["language"])
	}

	status := postLoganneEventWithBody(t, fmt.Sprintf(
		`{"type": "itemMerged", "sourceUri": %q, "targetUri": %q}`,
		sourceEntityURI, targetEntityURI,
	))
	assertEqual(t, "itemMerged: expected 204", 204, status)

	tagsAfter := getTagsForTrack(t, trackURL)
	if len(tagsAfter["language"]) == 0 {
		t.Fatal("language tag disappeared after itemMerged")
	}
	tagAfter := tagsAfter["language"][0]
	assertEqual(t, "itemMerged: URI should be rewritten to targetUri", targetEntityURI, tagAfter.URI)
	assertEqual(t, "itemMerged: name should be refreshed", "Target Entity Name", tagAfter.Name)
}

// ── Test 16: itemMerged with no matching tags is a no-op ─────────────────────

func TestItemMergedNoMatchingTagsIsNoop(t *testing.T) {
	clearData()
	withMockNameFetcher(t, map[string]string{targetEntityURI: "Target Entity Name"})

	status := postLoganneEventWithBody(t, fmt.Sprintf(
		`{"type": "itemMerged", "sourceUri": %q, "targetUri": %q}`,
		"https://eolas.l42.eu/metadata/person/nonexistent-source/", targetEntityURI,
	))
	assertEqual(t, "itemMerged no-match: expected 204", 204, status)
}

// ── Test 17: itemMerged is idempotent ────────────────────────────────────────

func TestItemMergedIsIdempotent(t *testing.T) {
	clearData()
	withMockNameFetcher(t, map[string]string{targetEntityURI: "Stable Target Name"})

	trackURL := createTrackWithURITag(t, "wh-item-merged-idempotent-1", sourceEntityURI)

	payload := fmt.Sprintf(
		`{"type": "itemMerged", "sourceUri": %q, "targetUri": %q}`,
		sourceEntityURI, targetEntityURI,
	)

	status1 := postLoganneEventWithBody(t, payload)
	assertEqual(t, "itemMerged idempotent: first delivery expected 204", 204, status1)

	status2 := postLoganneEventWithBody(t, payload)
	assertEqual(t, "itemMerged idempotent: second delivery expected 204", 204, status2)

	tagsAfter := getTagsForTrack(t, trackURL)
	if len(tagsAfter["language"]) == 0 {
		t.Fatal("language tag disappeared after idempotent itemMerged")
	}
	tagAfter := tagsAfter["language"][0]
	assertEqual(t, "itemMerged idempotent: URI should be stable", targetEntityURI, tagAfter.URI)
	assertEqual(t, "itemMerged idempotent: name should be stable", "Stable Target Name", tagAfter.Name)
}

// ── Name-fetch-failure tests ─────────────────────────────────────────────────
//
// When the entity name fetcher fails, the URI rewrite must still happen — the
// stale URI is no longer valid regardless of eolas availability. The stored
// name is left as-is and will be corrected by the daily reconciler.

// ── Test 18: contactLinked rewrites URI even when name fetch fails ────────────

func TestContactLinkedRewritesURIEvenWhenNameFetchFails(t *testing.T) {
	clearData()
	// Mock returns an error for every URI — simulates eolas being unavailable.
	orig := entityNameFetcher
	entityNameFetcher = func(uri string) (string, error) {
		return "", fmt.Errorf("eolas unavailable (mock)")
	}
	t.Cleanup(func() { entityNameFetcher = orig })

	trackURL := createTrackWithURITag(t, "wh-cl-name-fail-1", contactURI)
	originalTags := getTagsForTrack(t, trackURL)
	originalName := originalTags["language"][0].Name

	status := postLoganneEventWithBody(t, fmt.Sprintf(
		`{"type": "contactLinked", "contactUri": %q, "eolasUri": %q, "previousEolasUri": null}`,
		contactURI, eolasPersonURI,
	))
	assertEqual(t, "contactLinked name-fetch-fail: expected 204", 204, status)

	tagsAfter := getTagsForTrack(t, trackURL)
	if len(tagsAfter["language"]) == 0 {
		t.Fatal("language tag disappeared after contactLinked with name-fetch failure")
	}
	tagAfter := tagsAfter["language"][0]
	// URI must be rewritten even though name fetch failed
	assertEqual(t, "contactLinked name-fetch-fail: URI must be rewritten", eolasPersonURI, tagAfter.URI)
	// Name should be unchanged (preserved from before the event)
	assertEqual(t, "contactLinked name-fetch-fail: name should be unchanged", originalName, tagAfter.Name)
}

// ── Test 19: itemMerged rewrites URI even when name fetch fails ───────────────

func TestItemMergedRewritesURIEvenWhenNameFetchFails(t *testing.T) {
	clearData()
	orig := entityNameFetcher
	entityNameFetcher = func(uri string) (string, error) {
		return "", fmt.Errorf("eolas unavailable (mock)")
	}
	t.Cleanup(func() { entityNameFetcher = orig })

	trackURL := createTrackWithURITag(t, "wh-im-name-fail-1", sourceEntityURI)
	originalTags := getTagsForTrack(t, trackURL)
	originalName := originalTags["language"][0].Name

	status := postLoganneEventWithBody(t, fmt.Sprintf(
		`{"type": "itemMerged", "sourceUri": %q, "targetUri": %q}`,
		sourceEntityURI, targetEntityURI,
	))
	assertEqual(t, "itemMerged name-fetch-fail: expected 204", 204, status)

	tagsAfter := getTagsForTrack(t, trackURL)
	if len(tagsAfter["language"]) == 0 {
		t.Fatal("language tag disappeared after itemMerged with name-fetch failure")
	}
	tagAfter := tagsAfter["language"][0]
	// URI must be rewritten even though name fetch failed
	assertEqual(t, "itemMerged name-fetch-fail: URI must be rewritten", targetEntityURI, tagAfter.URI)
	// Name should be unchanged (preserved from before the event)
	assertEqual(t, "itemMerged name-fetch-fail: name should be unchanged", originalName, tagAfter.Name)
}
