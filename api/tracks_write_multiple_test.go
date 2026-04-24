package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"testing"
)

// getTagValue is a test helper that GETs a track by fingerprint and returns
// the first value of the named predicate, or "" if not present.
func getTagValue(test *testing.T, fingerprint, predicate string) string {
	req := basicRequest(test, "GET", "/v3/tracks?fingerprint="+fingerprint, "")
	resp, _ := doRawRequest(test, req)
	var track map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&track)
	tags, ok := track["tags"].(map[string]interface{})
	if !ok {
		return ""
	}
	arr, ok := tags[predicate].([]interface{})
	if !ok || len(arr) == 0 {
		return ""
	}
	obj, ok := arr[0].(map[string]interface{})
	if !ok {
		return ""
	}
	name, _ := obj["name"].(string)
	return name
}

/**
 * Checks that tracks can be updated in bulk based on a predicate.
 */
func TestPatchTracksByPredicate(test *testing.T) {
	clearData()

	setupRequest(test, "PUT", "/v3/tracks?fingerprint=abc1", `{"url":"http://example.org/track1", "duration": 7,"tags":{"artist":[{"name":"The Beatles"}],"title":[{"name":"Yellow Submarine"}]}}`, 200)
	setupRequest(test, "PUT", "/v3/tracks?fingerprint=abc2", `{"url":"http://example.org/track2", "duration": 7,"tags":{"artist":[{"name":"The Ladybirds"}],"title":[{"name":"Want to visit a Yellow Submarine"}]}}`, 200)
	setupRequest(test, "PUT", "/v3/tracks?fingerprint=abc3", `{"url":"http://example.org/track3", "duration": 7,"tags":{"artist":[{"name":"Yellow Submarine"}],"title":[{"name":"Love Me Do"}]}}`, 200)
	setupRequest(test, "PUT", "/v3/tracks?fingerprint=abc4", `{"url":"http://example.org/track4", "duration": 7,"tags":{"artist":[{"name":"Robert Johnson"}],"title":[{"name":"Sweet Home Chicago"}],"genre":[{"name":"blues"}]}}`, 200)
	setupRequest(test, "PUT", "/v3/tracks?fingerprint=abc5", `{"url":"http://example.org/track5", "duration": 7,"tags":{"artist":[{"name":"Panpipes Cover Band"}],"title":[{"name":"Yellow Submarine"}],"genre":[{"name":"panpipes"}]}}`, 200)

	request := basicRequest(test, "PATCH", "/v3/tracks?p.title=Yellow%20Submarine", `{"tags":{"genre":[{"name":"Maritime Songs"}]}}`)
	resp, _ := doRawRequest(test, request)
	if resp.StatusCode != 200 {
		test.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	// Only tracks 1 and 5 have exactly "Yellow Submarine" as title — they should be updated
	assertEqual(test, "Track 1 genre", "Maritime Songs", getTagValue(test, "abc1", "genre"))
	assertEqual(test, "Track 5 genre", "Maritime Songs", getTagValue(test, "abc5", "genre"))

	// Track 4 (no title match) should be unchanged
	assertEqual(test, "Track 4 genre", "blues", getTagValue(test, "abc4", "genre"))
}

/**
 * Checks that bulk update with If-None-Match: * only sets tags that are missing
 */
func TestPatchTracksMissingByPredicate(test *testing.T) {
	clearData()

	// Track without genre
	setupRequest(test, "PUT", "/v3/tracks?fingerprint=abc1", `{"url":"http://example.org/track1", "duration": 7,"tags":{"artist":[{"name":"The Beatles"}],"title":[{"name":"Yellow Submarine"}]}}`, 200)
	// Track without genre
	setupRequest(test, "PUT", "/v3/tracks?fingerprint=abc2", `{"url":"http://example.org/track2", "duration": 7,"tags":{"artist":[{"name":"The Ladybirds"}],"title":[{"name":"Yellow Submarine"}]}}`, 200)
	// Track with genre already set
	setupRequest(test, "PUT", "/v3/tracks?fingerprint=abc3", `{"url":"http://example.org/track3", "duration": 7,"tags":{"artist":[{"name":"Panpipes"}],"title":[{"name":"Yellow Submarine"}],"genre":[{"name":"panpipes"}]}}`, 200)

	request := basicRequest(test, "PATCH", "/v3/tracks?p.title=Yellow%20Submarine", `{"tags":{"genre":[{"name":"Maritime Songs"}]}}`)
	request.Header.Add("If-None-Match", "*")
	resp, _ := doRawRequest(test, request)
	if resp.StatusCode != 200 {
		test.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	// Tracks 1 and 2 (missing genre) should have been updated
	assertEqual(test, "Track 1 genre", "Maritime Songs", getTagValue(test, "abc1", "genre"))
	assertEqual(test, "Track 2 genre", "Maritime Songs", getTagValue(test, "abc2", "genre"))

	// Track 3 (already had genre) should be unchanged
	assertEqual(test, "Track 3 genre", "panpipes", getTagValue(test, "abc3", "genre"))
}

/**
 * Checks that bulk update works with multiple predicate filters
 */
func TestPatchTracksByMultiplePredicates(test *testing.T) {
	clearData()

	setupRequest(test, "PUT", "/v3/tracks?fingerprint=abc1", `{"url":"http://example.org/track1", "duration": 7,"tags":{"artist":[{"name":"The Beatles"}],"title":[{"name":"Yellow Submarine"}]}}`, 200)
	setupRequest(test, "PUT", "/v3/tracks?fingerprint=abc2", `{"url":"http://example.org/track2", "duration": 7,"tags":{"artist":[{"name":"Panpipes Cover Band"}],"title":[{"name":"Yellow Submarine"}]}}`, 200)
	setupRequest(test, "PUT", "/v3/tracks?fingerprint=abc3", `{"url":"http://example.org/track3", "duration": 7,"tags":{"artist":[{"name":"The Beatles"}],"title":[{"name":"Hey Jude"}]}}`, 200)

	request := basicRequest(test, "PATCH", "/v3/tracks?p.title=Yellow%20Submarine&p.artist=Panpipes%20Cover%20Band", `{"tags":{"genre":[{"name":"Maritime Songs"}]}}`)
	resp, _ := doRawRequest(test, request)

	var result SearchResultV3
	json.NewDecoder(resp.Body).Decode(&result)
	if len(result.Tracks) != 1 {
		test.Errorf("Expected 1 track to be updated, got %d", len(result.Tracks))
	}
	if len(result.Tracks) > 0 && result.Tracks[0].ID != 2 {
		test.Errorf("Expected track 2 to be updated, got track %d", result.Tracks[0].ID)
	}

	assertEqual(test, "Track 2 genre", "Maritime Songs", getTagValue(test, "abc2", "genre"))
	// Track 1 (wrong artist) should be unchanged
	if getTagValue(test, "abc1", "genre") != "" {
		test.Error("Track 1 should not have genre after multi-predicate patch")
	}
}

/**
 * Checks that bulk PATCH with q= parameter is rejected with a 400 error
 */
func TestPatchTracksByQueryReturnsError(test *testing.T) {
	makeRequest(test, "PATCH", "/v3/tracks?q=Submarine", `{"tags":{"genre":[{"name":"Nautical"}]}}`, 400, `{"error":"The q parameter is no longer supported. Use p. parameters for predicate-based search.","code":"bad_request"}`, true)
}

/**
 * Checks that PUT on a bulk query returns 405 Method Not Allowed
 */
func TestCantPutInBulk(test *testing.T) {
	makeRequestWithUnallowedMethod(test, "/v3/tracks?p.title=Yellow%20Submarine", "PUT", []string{"PATCH", "GET"})
}

/**
 * Checks that bulk update rejects attempts to change unique identifier fields
 */
func TestCantBulkUpdateIdentifiers(test *testing.T) {
	makeRequest(test, "PATCH", "/v3/tracks?p.title=Yellow", `{"url": "http://example.org/track7"}`, 400, `{"error":"Can't bulk update url","code":"bad_request"}`, true)
	makeRequest(test, "PATCH", "/v3/tracks?p.title=Blue", `{"id": 7}`, 400, `{"error":"Can't bulk update id","code":"bad_request"}`, true)
	makeRequest(test, "PATCH", "/v3/tracks?p.title=Green", `{"fingerprint": "abc7"}`, 400, `{"error":"Can't bulk update fingerprint","code":"bad_request"}`, true)
}

/**
 * Checks bulk duration change fires correct action and Loganne event
 */
func TestBulkUpdateDuration(test *testing.T) {
	clearData()

	setupRequest(test, "PUT", "/v3/tracks?fingerprint=abc1", `{"url":"http://example.org/track1", "duration": 7,"tags":{"title":[{"name":"Yellow Submarine"}]}}`, 200)
	setupRequest(test, "PUT", "/v3/tracks?fingerprint=abc2", `{"url":"http://example.org/track2", "duration": 7,"tags":{"title":[{"name":"Hey Jude"}]}}`, 200)

	restartServer()
	request := basicRequest(test, "PATCH", "/v3/tracks?p.title=Yellow%20Submarine", `{"duration": 137}`)
	response, _ := doRawRequest(test, request)
	checkResponseHeader(test, response, "Track-Action", "tracksUpdated")
	assertEqual(test, "Loganne call", "tracksUpdated", lastLoganneType)

	checkDuration := func(fingerprint string, expectedDuration float64) {
		req := basicRequest(test, "GET", "/v3/tracks?fingerprint="+fingerprint, "")
		resp, _ := doRawRequest(test, req)
		var track map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&track)
		if dur, ok := track["duration"].(float64); !ok || dur != expectedDuration {
			test.Errorf("fingerprint=%s: expected duration %v, got %v", fingerprint, expectedDuration, track["duration"])
		}
	}

	checkDuration("abc1", 137)
	checkDuration("abc2", 7) // unchanged
}

/**
 * Checks bulk update pagination — only the page-2 tracks are updated, result is paginated
 */
func TestBulkUpdatePagination(test *testing.T) {
	clearData()

	for i := 1; i <= 45; i++ {
		id := fmt.Sprintf("%d", i)
		escapedURL := url.QueryEscape("http://example.org/track/id" + id)
		setupRequest(test, "PUT", fmt.Sprintf("/v3/tracks?url=%s", escapedURL), `{"fingerprint": "fp`+id+`", "duration": 7,"tags":{"title":[{"name":"Yellow Submarine"}]}}`, 200)
		setupRequest(test, "PUT", "/v3/tracks/"+id+"/weighting", "4.3", 200)
	}

	restartServer()
	request := basicRequest(test, "PATCH", "/v3/tracks?p.title=Yellow%20Submarine&page=2", `{"tags":{"genre":[{"name":"Maritime Songs"}]}}`)
	resp, _ := doRawRequest(test, request)

	var result SearchResultV3
	json.NewDecoder(resp.Body).Decode(&result)
	if len(result.Tracks) != 20 {
		test.Errorf("Expected 20 tracks on page 2, got %d", len(result.Tracks))
	}
	if result.TotalPages != 3 {
		test.Errorf("Expected 3 total pages, got %d", result.TotalPages)
	}

	checkHasGenre := func(trackID int, expectGenre bool) {
		req := basicRequest(test, "GET", fmt.Sprintf("/v3/tracks/%d", trackID), "")
		respG, _ := doRawRequest(test, req)
		var track map[string]interface{}
		json.NewDecoder(respG.Body).Decode(&track)
		tags := track["tags"].(map[string]interface{})
		genreArr, hasGenre := tags["genre"].([]interface{})
		hasGenreVal := hasGenre && len(genreArr) > 0
		if expectGenre && !hasGenreVal {
			test.Errorf("Track %d should have genre but doesn't", trackID)
		}
		if !expectGenre && hasGenreVal {
			test.Errorf("Track %d should NOT have genre but does", trackID)
		}
	}

	// Track on page 2 (track 27) should be updated
	checkHasGenre(27, true)
	// Track on page 1 (track 20) should NOT be updated
	checkHasGenre(20, false)
	// Track on page 3 (track 41) should NOT be updated
	checkHasGenre(41, false)
}

/**
 * Checks bulk update with page=all returns all matching tracks and updates all of them
 */
func TestBulkUpdateNoPagination(test *testing.T) {
	clearData()

	for i := 1; i <= 45; i++ {
		id := fmt.Sprintf("%d", i)
		escapedURL := url.QueryEscape("http://example.org/track/id" + id)
		setupRequest(test, "PUT", fmt.Sprintf("/v3/tracks?url=%s", escapedURL), `{"fingerprint": "fp`+id+`", "duration": 7,"tags":{"title":[{"name":"Yellow Submarine"}]}}`, 200)
		setupRequest(test, "PUT", "/v3/tracks/"+id+"/weighting", "4.3", 200)
	}

	restartServer()
	request := basicRequest(test, "PATCH", "/v3/tracks?p.title=Yellow%20Submarine&page=all", `{"tags":{"genre":[{"name":"Maritime Songs"}]}}`)
	resp, _ := doRawRequest(test, request)

	var result SearchResultV3
	json.NewDecoder(resp.Body).Decode(&result)
	if len(result.Tracks) != 45 {
		test.Errorf("Expected all 45 tracks with page=all, got %d", len(result.Tracks))
	}

	// Spot-check tracks on different pages are all updated
	for _, trackID := range []int{1, 20, 27, 41, 45} {
		req := basicRequest(test, "GET", fmt.Sprintf("/v3/tracks/%d", trackID), "")
		respG, _ := doRawRequest(test, req)
		var track map[string]interface{}
		json.NewDecoder(respG.Body).Decode(&track)
		tags := track["tags"].(map[string]interface{})
		genreArr, hasGenre := tags["genre"].([]interface{})
		if !hasGenre || len(genreArr) == 0 {
			test.Errorf("Track %d should have genre after page=all patch, but doesn't", trackID)
			continue
		}
		if genreArr[0].(map[string]interface{})["name"] != "Maritime Songs" {
			test.Errorf("Track %d: expected genre 'Maritime Songs', got %v", trackID, genreArr[0])
		}
	}
}

// TestBulkPatchTagOnlyFiresLoganne checks that a bulk PATCH that changes only tags
// (no scalar fields) fires a Loganne tracksUpdated event and returns Track-Action: tracksUpdated.
func TestBulkPatchTagOnlyFiresLoganne(test *testing.T) {
	clearData()
	setupRequest(test, "PUT", "/v3/tracks?fingerprint=tagfire1", `{"url":"http://example.org/tagfire/1","duration":7,"tags":{"title":[{"name":"Yellow Submarine"}]}}`, 200)
	setupRequest(test, "PUT", "/v3/tracks?fingerprint=tagfire2", `{"url":"http://example.org/tagfire/2","duration":7,"tags":{"title":[{"name":"Yellow Submarine"}]}}`, 200)
	loganneRequestCount = 0

	request := basicRequest(test, "PATCH", "/v3/tracks?p.title=Yellow%20Submarine", `{"tags":{"genre":[{"name":"Maritime Songs"}]}}`)
	response, _ := doRawRequest(test, request)

	checkResponseHeader(test, response, "Track-Action", "tracksUpdated")
	assertEqual(test, "Loganne event type", "tracksUpdated", lastLoganneType)
	assertEqual(test, "Loganne request count", 1, loganneRequestCount)
}

// TestBulkPatchTagOnlyNoChangeIsNoOp checks that a bulk PATCH with identical tag values
// does not fire Loganne and returns Track-Action: noChange.
func TestBulkPatchTagOnlyNoChangeIsNoOp(test *testing.T) {
	clearData()
	setupRequest(test, "PUT", "/v3/tracks?fingerprint=tagnoop1", `{"url":"http://example.org/tagnoop/1","duration":7,"tags":{"title":[{"name":"Yellow Submarine"}],"genre":[{"name":"Maritime Songs"}]}}`, 200)
	loganneRequestCount = 0

	request := basicRequest(test, "PATCH", "/v3/tracks?p.title=Yellow%20Submarine", `{"tags":{"genre":[{"name":"Maritime Songs"}]}}`)
	response, _ := doRawRequest(test, request)

	checkResponseHeader(test, response, "Track-Action", "noChange")
	assertEqual(test, "Loganne request count after no-op", 0, loganneRequestCount)
}

// TestBulkPatchMixedScalarAndTagsNoExtraLoganne checks that mixing scalar and tag
// changes does not fire an extra Loganne event for the tags on top of the scalar events.
// The scalar path fires one individual trackUpdated and one bulk tracksUpdated (count=2).
// Tags must not add a third event.
func TestBulkPatchMixedScalarAndTagsNoExtraLoganne(test *testing.T) {
	clearData()
	setupRequest(test, "PUT", "/v3/tracks?fingerprint=mixedone1", `{"url":"http://example.org/mixedone/1","duration":7,"tags":{"title":[{"name":"Yellow Submarine"}]}}`, 200)
	loganneRequestCount = 0

	request := basicRequest(test, "PATCH", "/v3/tracks?p.title=Yellow%20Submarine", `{"duration":137,"tags":{"genre":[{"name":"Maritime Songs"}]}}`)
	response, _ := doRawRequest(test, request)

	checkResponseHeader(test, response, "Track-Action", "tracksUpdated")
	assertEqual(test, "Loganne event type", "tracksUpdated", lastLoganneType)
	// Scalar path fires 2 events (trackUpdated + tracksUpdated); tags must not add a third.
	assertEqual(test, "Loganne request count (expect 2, not 3)", 2, loganneRequestCount)
}
