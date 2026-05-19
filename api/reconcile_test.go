package main

import (
	"testing"
)

// ── reconcileTagNames unit tests ─────────────────────────────────────────────

// ── Test: reconciliation fetches and updates eolas URIs ──────────────────────

func TestReconcileUpdatesEolasURIs(t *testing.T) {
	clearData()

	eolasEntityURI := "https://eolas.l42.eu/metadata/person/old-eolas/"

	// Create one track tagged with an eolas URI.
	eolasTrackURL := createTrackWithURITag(t, "reconcile-eolas-1", eolasEntityURI)

	// Open a second handle to the same DB to call reconcileTagNamesWithFetchers directly.
	store := DBInit("testrouting.sqlite", MockLoganne{})
	store.reconcileTagNamesWithFetchers(
		func(uris []string) map[string]string {
			result := make(map[string]string)
			for _, u := range uris {
				if u == eolasEntityURI {
					result[u] = "Updated Eolas Name"
				}
			}
			return result
		},
	)

	// Eolas tag name should be updated.
	eolasTagsResult := getTagsForTrack(t, eolasTrackURL)
	if len(eolasTagsResult["language"]) == 0 {
		t.Fatal("eolas track has no language tag after reconcile")
	}
	assertEqual(t, "eolas tag name after reconcile", "Updated Eolas Name", eolasTagsResult["language"][0].Name)
	assertEqual(t, "eolas URI unchanged after reconcile", eolasEntityURI, eolasTagsResult["language"][0].URI)
}

// ── Test: reconciliation skips URIs from unrecognised hosts ──────────────────

func TestReconcileSkipsUnknownHostURIs(t *testing.T) {
	clearData()

	eolasEntityURI2 := "https://eolas.l42.eu/metadata/person/recon-skip/"
	trackURL := createTrackWithURITag(t, "reconcile-skip-1", eolasEntityURI2)

	// Override the URI to an unrecognised host directly in the DB.
	store := DBInit("testrouting.sqlite", MockLoganne{})
	store.DB.MustExec(`UPDATE tag SET uri = 'https://unknown.example.com/thing/1' WHERE uri = ?`, eolasEntityURI2)

	eolasCallCount := 0
	store.reconcileTagNamesWithFetchers(
		func(uris []string) map[string]string {
			eolasCallCount += len(uris)
			return nil
		},
	)

	// The unknown-host URI should not have been passed to the eolas fetcher.
	if eolasCallCount != 0 {
		t.Errorf("expected no fetcher calls for unknown-host URI; eolas=%d", eolasCallCount)
	}

	// The tag row itself should be untouched.
	tagsAfter := getTagsForTrack(t, trackURL)
	if len(tagsAfter["language"]) == 0 {
		t.Fatal("language tag disappeared after reconcile")
	}
	assertEqual(t, "unknown-host URI unchanged", "https://unknown.example.com/thing/1", tagsAfter["language"][0].URI)
}

// ── Test: reconciliation is a no-op when there are no URI tags ───────────────

func TestReconcileNoURITags(t *testing.T) {
	clearData()
	store := DBInit("testrouting.sqlite", MockLoganne{})

	called := false
	store.reconcileTagNamesWithFetchers(
		func(uris []string) map[string]string { called = true; return nil },
	)

	if called {
		t.Error("expected no fetcher calls when there are no URI tags")
	}
}
