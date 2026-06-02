package main

import (
	"testing"
)

func makeTagOnlyChangeSet(predicateID string) TrackV3 {
	return TrackV3{
		Tags: map[string][]TagValueV3{
			predicateID: {{Name: "2026-05-10T12:00:00Z"}},
		},
	}
}

func TestGetBespokeLoganneMessageLastSuccessfulPlay(test *testing.T) {
	msg, _ := getBespokeLoganneMessage(makeTagOnlyChangeSet("lastSuccessfulPlay"), Track{}, `"Tuesday's Gone"`)
	assertEqual(test, "bespoke loganne message for lastSuccessfulPlay", `Track "Tuesday's Gone" finished playing`, msg)
}

func TestGetBespokeLoganneLevelLastSuccessfulPlay(test *testing.T) {
	_, level := getBespokeLoganneMessage(makeTagOnlyChangeSet("lastSuccessfulPlay"), Track{}, `"Tuesday's Gone"`)
	assertEqual(test, "bespoke loganne level for lastSuccessfulPlay", "detail", level)
}

func TestGetBespokeLoganneLevelLastSkip(test *testing.T) {
	// lastSkip has no LoganneLevel set — should default to "routine"
	_, level := getBespokeLoganneMessage(makeTagOnlyChangeSet("lastSkip"), Track{}, `"Tuesday's Gone"`)
	assertEqual(test, "bespoke loganne level for lastSkip (default)", "routine", level)
}

func TestGetBespokeLoganneMessageLastError(test *testing.T) {
	msg, _ := getBespokeLoganneMessage(makeTagOnlyChangeSet("lastError"), Track{}, `"Tuesday's Gone"`)
	assertEqual(test, "bespoke loganne message for lastError", `Track "Tuesday's Gone" errored`, msg)
}

func TestGetBespokeLoganneMessageLastSkip(test *testing.T) {
	msg, _ := getBespokeLoganneMessage(makeTagOnlyChangeSet("lastSkip"), Track{}, `"Tuesday's Gone"`)
	assertEqual(test, "bespoke loganne message for lastSkip", `Track "Tuesday's Gone" skipped`, msg)
}

func TestGetBespokeLoganneMessageMultipleTagsReturnsEmpty(test *testing.T) {
	changeSet := TrackV3{
		Tags: map[string][]TagValueV3{
			"lastSuccessfulPlay": {{Name: "2026-05-10T12:00:00Z"}},
			"lastSkip":           {{Name: "2026-05-10T11:00:00Z"}},
		},
	}
	msg, _ := getBespokeLoganneMessage(changeSet, Track{}, `"Tuesday's Gone"`)
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
	msg, _ := getBespokeLoganneMessage(changeSet, existing, `"Tuesday's Gone"`)
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
	msg, _ := getBespokeLoganneMessage(changeSet, existing, `"Tuesday's Gone"`)
	assertEqual(test, "bespoke loganne message when fingerprint is unchanged", `Track "Tuesday's Gone" finished playing`, msg)
}

func TestGetBespokeLoganneMessageUnknownTagReturnsEmpty(test *testing.T) {
	msg, _ := getBespokeLoganneMessage(makeTagOnlyChangeSet("title"), Track{}, "#42")
	assertEqual(test, "bespoke loganne message for non-bespoke predicate", "", msg)
}
