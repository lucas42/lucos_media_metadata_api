package main

import (
	"testing"
)

func TestMultiValuePredicates(test *testing.T) {
	expectedMultiValue := []string{"composer", "producer", "language", "offence", "about", "mentions"}
	for _, pred := range expectedMultiValue {
		if !IsMultiValue(pred) {
			test.Errorf("Expected predicate %q to be multi-value", pred)
		}
	}
}

func TestSingleValuePredicates(test *testing.T) {
	singleValue := []string{"title", "artist", "album", "duration", "img"}
	for _, pred := range singleValue {
		if IsMultiValue(pred) {
			test.Errorf("Expected predicate %q to be single-value", pred)
		}
	}
}

func TestGetPredicateConfigKnown(test *testing.T) {
	config := GetPredicateConfig("composer")
	if !config.MultiValue {
		test.Error("Expected composer config to have MultiValue true")
	}
}

func TestGetPredicateConfigUnknown(test *testing.T) {
	config := GetPredicateConfig("nonexistent")
	if config.MultiValue {
		test.Error("Expected unknown predicate config to have MultiValue false")
	}
}

func TestRegistryCount(test *testing.T) {
	count := 0
	for _, config := range predicateRegistry {
		if config.MultiValue {
			count++
		}
	}
	if count != 6 {
		test.Errorf("Expected 6 multi-value predicates, got %d (album is single-value, so count unchanged)", count)
	}
}

func TestRequiresURIPredicates(test *testing.T) {
	expectedRequiresURI := []string{"language", "about", "mentions", "album"}
	for _, pred := range expectedRequiresURI {
		config := GetPredicateConfig(pred)
		if !config.RequiresURI {
			test.Errorf("Expected predicate %q to have RequiresURI true", pred)
		}
	}
}

func TestNonURIPredicatesDoNotRequireURI(test *testing.T) {
	nonURIPredicates := []string{"composer", "producer", "offence", "title", "artist"}
	for _, pred := range nonURIPredicates {
		config := GetPredicateConfig(pred)
		if config.RequiresURI {
			test.Errorf("Expected predicate %q to have RequiresURI false", pred)
		}
	}
}

func TestGetRequiresURIPredicates(test *testing.T) {
	predicates := GetRequiresURIPredicates()
	if len(predicates) != 4 {
		test.Errorf("Expected 4 RequiresURI predicates, got %d: %v", len(predicates), predicates)
	}
	predicateSet := make(map[string]bool)
	for _, p := range predicates {
		predicateSet[p] = true
	}
	for _, expected := range []string{"language", "about", "mentions", "album"} {
		if !predicateSet[expected] {
			test.Errorf("Expected %q in GetRequiresURIPredicates result, got %v", expected, predicates)
		}
	}
}

func makeTagOnlyChangeSet(predicateID string) TrackV3 {
	return TrackV3{
		Tags: map[string][]TagValueV3{
			predicateID: {{Name: "2026-05-10T12:00:00Z"}},
		},
	}
}

func TestGetBespokeLoganneMessageLastSuccessfulPlay(test *testing.T) {
	msg := GetBespokeLoganneMessage(makeTagOnlyChangeSet("lastSuccessfulPlay"), Track{}, `"Tuesday's Gone"`)
	assertEqual(test, "bespoke loganne message for lastSuccessfulPlay", `Track "Tuesday's Gone" played`, msg)
}

func TestGetBespokeLoganneMessageLastError(test *testing.T) {
	msg := GetBespokeLoganneMessage(makeTagOnlyChangeSet("lastError"), Track{}, `"Tuesday's Gone"`)
	assertEqual(test, "bespoke loganne message for lastError", `Track "Tuesday's Gone" errored`, msg)
}

func TestGetBespokeLoganneMessageLastSkip(test *testing.T) {
	msg := GetBespokeLoganneMessage(makeTagOnlyChangeSet("lastSkip"), Track{}, `"Tuesday's Gone"`)
	assertEqual(test, "bespoke loganne message for lastSkip", `Track "Tuesday's Gone" skipped`, msg)
}

func TestGetBespokeLoganneMessageMultipleTagsReturnsEmpty(test *testing.T) {
	changeSet := TrackV3{
		Tags: map[string][]TagValueV3{
			"lastSuccessfulPlay": {{Name: "2026-05-10T12:00:00Z"}},
			"lastSkip":           {{Name: "2026-05-10T11:00:00Z"}},
		},
	}
	msg := GetBespokeLoganneMessage(changeSet, Track{}, `"Tuesday's Gone"`)
	assertEqual(test, "bespoke loganne message for multi-tag changeset", "", msg)
}

func TestGetBespokeLoganneMessageWithChangedFingerprintReturnsEmpty(test *testing.T) {
	changeSet := TrackV3{
		Fingerprint: "new-fingerprint",
		Tags: map[string][]TagValueV3{
			"lastSuccessfulPlay": {{Name: "2026-05-10T12:00:00Z"}},
		},
	}
	existing := Track{Fingerprint: "old-fingerprint"}
	msg := GetBespokeLoganneMessage(changeSet, existing, `"Tuesday's Gone"`)
	assertEqual(test, "bespoke loganne message when fingerprint changes", "", msg)
}

func TestGetBespokeLoganneMessageWithSameFingerprintNotBlocked(test *testing.T) {
	// The request handler sets filter fields on the changeset even for tag-only PATCHes;
	// same-value scalars must not suppress the bespoke message.
	changeSet := TrackV3{
		Fingerprint: "abc123",
		Tags: map[string][]TagValueV3{
			"lastSuccessfulPlay": {{Name: "2026-05-10T12:00:00Z"}},
		},
	}
	existing := Track{Fingerprint: "abc123"}
	msg := GetBespokeLoganneMessage(changeSet, existing, `"Tuesday's Gone"`)
	assertEqual(test, "bespoke loganne message when fingerprint is unchanged", `Track "Tuesday's Gone" played`, msg)
}

func TestGetBespokeLoganneMessageUnknownTagReturnsEmpty(test *testing.T) {
	msg := GetBespokeLoganneMessage(makeTagOnlyChangeSet("title"), Track{}, "#42")
	assertEqual(test, "bespoke loganne message for non-bespoke predicate", "", msg)
}
