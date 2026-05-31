package main

import (
	"fmt"
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
		func(uris []string) (map[string]string, error) {
			result := make(map[string]string)
			for _, u := range uris {
				if u == eolasEntityURI {
					result[u] = "Updated Eolas Name"
				}
			}
			return result, nil
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
		func(uris []string) (map[string]string, error) {
			eolasCallCount += len(uris)
			return nil, nil
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
		func(uris []string) (map[string]string, error) { called = true; return nil, nil },
	)

	if called {
		t.Error("expected no fetcher calls when there are no URI tags")
	}
}

// ── Test: a failed eolas fetch fails the whole job (does not silently no-op) ──
// Regression guard for #303: when the eolas call itself fails, reconcile must
// return an error (so reconcileTagNames reports failure to schedule_tracker)
// rather than swallowing it and reporting success.

func TestReconcileFailsWhenEolasFetchErrors(t *testing.T) {
	clearData()

	eolasEntityURI := "https://eolas.l42.eu/metadata/person/recon-fetch-err/"
	trackURL := createTrackWithURITag(t, "reconcile-fetch-err-1", eolasEntityURI)

	store := DBInit("testrouting.sqlite", MockLoganne{})
	err := store.reconcileTagNamesWithFetchers(
		func(uris []string) (map[string]string, error) {
			return nil, fmt.Errorf("context deadline exceeded")
		},
	)

	if err == nil {
		t.Fatal("expected reconcileTagNamesWithFetchers to return an error when the eolas fetch fails")
	}

	// The tag must be left untouched — a failed fetch resolves nothing.
	tagsAfter := getTagsForTrack(t, trackURL)
	if len(tagsAfter["language"]) == 0 {
		t.Fatal("language tag disappeared after a failed reconcile")
	}
	assertEqual(t, "URI unchanged after failed fetch", eolasEntityURI, tagsAfter["language"][0].URI)
}

// ── Test: a successful fetch resolving zero names is NOT a failure ───────────
// Per the #303 refinement: the trigger is the failed call, not the resolved
// count. A fetch that succeeds but matches no names returns no error.

func TestReconcileZeroResolvedIsNotAnError(t *testing.T) {
	clearData()

	eolasEntityURI := "https://eolas.l42.eu/metadata/person/recon-zero/"
	createTrackWithURITag(t, "reconcile-zero-1", eolasEntityURI)

	store := DBInit("testrouting.sqlite", MockLoganne{})
	err := store.reconcileTagNamesWithFetchers(
		func(uris []string) (map[string]string, error) {
			// Successful call, but nothing matched.
			return map[string]string{}, nil
		},
	)

	if err != nil {
		t.Fatalf("expected no error when the fetch succeeds with zero resolved names, got: %v", err)
	}
}

// ── Test: missing eolas credentials cause an error (not a silent skip) ───────
// Per lucas42's review of #303: a lack of credentials should fail the fetch, so
// the reconcile job reports failure rather than silently no-opping. The key
// check returns before any HTTP call, so this makes no network request.

func TestFetchEolasNamesErrorsWithoutKey(t *testing.T) {
	t.Setenv("KEY_LUCOS_EOLAS", "")

	_, err := fetchEolasNames([]string{"https://eolas.l42.eu/metadata/person/1/"})
	if err == nil {
		t.Fatal("expected fetchEolasNames to return an error when KEY_LUCOS_EOLAS is unset")
	}
}
