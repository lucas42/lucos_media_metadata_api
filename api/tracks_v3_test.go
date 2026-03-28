package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
)

// doRawRequest is a helper that returns the raw response without checking body content.
func doRawRequest(t *testing.T, request *http.Request) (*http.Response, error) {
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Error(err)
	}
	if response.StatusCode >= 500 {
		t.Errorf("Error code %d returned for %s", response.StatusCode, request.URL.String())
	}
	return response, err
}

func TestV3GetTrackReturnsMultiValueAsArray(test *testing.T) {
	clearData()
	trackurl := "http://example.org/v3track/1"
	escapedTrackUrl := url.QueryEscape(trackurl)

	// Create track via v2 (v2 accepts language as a string)
	v2Path := fmt.Sprintf("/v2/tracks?url=%s", escapedTrackUrl)
	v2Input := `{"fingerprint": "v3test1", "duration": 200, "tags": {"title": "Test Song", "artist": "Test Artist", "language": "en"}}`
	request := basicRequest(test, "PUT", v2Path, v2Input)
	resp, _ := doRawRequest(test, request)
	if resp.StatusCode != 200 {
		test.Fatalf("Failed to create track via v2: %d", resp.StatusCode)
	}

	// GET via v3 should return multi-value predicates as arrays
	v3Path := fmt.Sprintf("/v3/tracks?url=%s", escapedTrackUrl)
	request2 := basicRequest(test, "GET", v3Path, "")
	resp2, _ := doRawRequest(test, request2)
	var track map[string]interface{}
	json.NewDecoder(resp2.Body).Decode(&track)
	tags := track["tags"].(map[string]interface{})

	// title is single-value -> string
	if _, ok := tags["title"].(string); !ok {
		test.Errorf("Expected title to be a string in v3 response, got %T", tags["title"])
	}
	// language is multi-value -> array (even with one value)
	if langArr, ok := tags["language"].([]interface{}); !ok {
		test.Errorf("Expected language to be an array in v3 response, got %T", tags["language"])
	} else if len(langArr) != 1 {
		test.Errorf("Expected 1 language value, got %d", len(langArr))
	} else {
		assertEqual(test, "language[0]", "en", langArr[0].(string))
	}
}

func TestV3PutTrackWithMultiValueTags(test *testing.T) {
	clearData()
	trackurl := "http://example.org/v3track/2"
	escapedTrackUrl := url.QueryEscape(trackurl)
	v3Path := fmt.Sprintf("/v3/tracks?url=%s", escapedTrackUrl)

	inputJSON := `{"fingerprint": "v3test2", "duration": 300, "tags": {"title": "Multi Test", "language": ["en", "fr"], "composer": ["A", "B"]}}`
	request := basicRequest(test, "PUT", v3Path, inputJSON)
	resp, _ := doRawRequest(test, request)
	if resp.StatusCode != 200 {
		test.Fatalf("Failed to PUT v3 track: %d", resp.StatusCode)
	}

	// Verify the round-trip
	request2 := basicRequest(test, "GET", v3Path, "")
	resp2, _ := doRawRequest(test, request2)
	var track map[string]interface{}
	json.NewDecoder(resp2.Body).Decode(&track)
	tags := track["tags"].(map[string]interface{})

	// Check title is string
	assertEqual(test, "title", "Multi Test", tags["title"].(string))

	// Check language is array with 2 elements
	langArr := tags["language"].([]interface{})
	if len(langArr) != 2 {
		test.Errorf("Expected 2 language values, got %d", len(langArr))
	}

	// Check composer is array with 2 elements
	composerArr := tags["composer"].([]interface{})
	if len(composerArr) != 2 {
		test.Errorf("Expected 2 composer values, got %d", len(composerArr))
	}
}

func TestV3PatchTrackUpdatesTags(test *testing.T) {
	clearData()
	trackurl := "http://example.org/v3track/3"
	escapedTrackUrl := url.QueryEscape(trackurl)
	v3Path := fmt.Sprintf("/v3/tracks?url=%s", escapedTrackUrl)

	// Create track
	createReq := basicRequest(test, "PUT", v3Path, `{"fingerprint": "v3test3", "duration": 200, "tags": {"title": "Original", "language": ["en"]}}`)
	resp, _ := doRawRequest(test, createReq)
	if resp.StatusCode != 200 {
		test.Fatalf("Failed to create track: %d", resp.StatusCode)
	}

	// PATCH to update language
	patchReq := basicRequest(test, "PATCH", v3Path, `{"tags": {"language": ["en", "de", "fr"]}}`)
	resp2, _ := doRawRequest(test, patchReq)
	if resp2.StatusCode != 200 {
		test.Fatalf("Failed to PATCH track: %d", resp2.StatusCode)
	}

	// Verify
	getReq := basicRequest(test, "GET", v3Path, "")
	resp3, _ := doRawRequest(test, getReq)
	var track map[string]interface{}
	json.NewDecoder(resp3.Body).Decode(&track)
	tags := track["tags"].(map[string]interface{})

	langArr := tags["language"].([]interface{})
	if len(langArr) != 3 {
		test.Errorf("Expected 3 language values after PATCH, got %d", len(langArr))
	}
}

func TestV3RejectsStringForMultiValuePredicate(test *testing.T) {
	clearData()
	trackurl := "http://example.org/v3track/4"
	escapedTrackUrl := url.QueryEscape(trackurl)
	v3Path := fmt.Sprintf("/v3/tracks?url=%s", escapedTrackUrl)

	// Should reject a plain string for a multi-value predicate
	request := basicRequest(test, "PUT", v3Path, `{"fingerprint": "v3test4", "duration": 200, "tags": {"language": "en"}}`)
	resp, _ := doRawRequest(test, request)
	if resp.StatusCode != 400 {
		test.Errorf("Expected 400 for string on multi-value predicate, got %d", resp.StatusCode)
	}
}

func TestV3RejectsArrayForSingleValuePredicate(test *testing.T) {
	clearData()
	trackurl := "http://example.org/v3track/5"
	escapedTrackUrl := url.QueryEscape(trackurl)
	v3Path := fmt.Sprintf("/v3/tracks?url=%s", escapedTrackUrl)

	// Should reject an array for a single-value predicate
	request := basicRequest(test, "PUT", v3Path, `{"fingerprint": "v3test5", "duration": 200, "tags": {"title": ["a", "b"]}}`)
	resp, _ := doRawRequest(test, request)
	if resp.StatusCode != 400 {
		test.Errorf("Expected 400 for array on single-value predicate, got %d", resp.StatusCode)
	}
}

func TestV3GetMultipleTracks(test *testing.T) {
	clearData()
	// Create two tracks via v3
	req1 := basicRequest(test, "PUT", "/v3/tracks?url="+url.QueryEscape("http://example.org/v3multi/1"), `{"fingerprint": "v3m1", "duration": 100, "tags": {"title": "Track A", "language": ["en"]}}`)
	doRawRequest(test, req1)
	req2 := basicRequest(test, "PUT", "/v3/tracks?url="+url.QueryEscape("http://example.org/v3multi/2"), `{"fingerprint": "v3m2", "duration": 200, "tags": {"title": "Track B", "language": ["fr", "de"]}}`)
	doRawRequest(test, req2)

	// GET list
	request := basicRequest(test, "GET", "/v3/tracks", "")
	resp, _ := doRawRequest(test, request)
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	tracks := result["tracks"].([]interface{})
	if len(tracks) != 2 {
		test.Errorf("Expected 2 tracks, got %d", len(tracks))
	}
	// Verify that tracks in list also use v3 format
	for _, trackRaw := range tracks {
		track := trackRaw.(map[string]interface{})
		tags := track["tags"].(map[string]interface{})
		if lang, ok := tags["language"]; ok {
			if _, isArr := lang.([]interface{}); !isArr {
				test.Errorf("Expected language in list track to be array, got %T", lang)
			}
		}
	}
}

func TestV2EndpointsUnchangedAfterV3(test *testing.T) {
	clearData()
	trackurl := "http://example.org/v2compat/1"
	escapedTrackUrl := url.QueryEscape(trackurl)

	// Create track via v3 with multi-value tags
	v3Path := fmt.Sprintf("/v3/tracks?url=%s", escapedTrackUrl)
	createReq := basicRequest(test, "PUT", v3Path, `{"fingerprint": "compat1", "duration": 200, "tags": {"title": "Compat Test", "language": ["en", "fr"]}}`)
	resp, _ := doRawRequest(test, createReq)
	if resp.StatusCode != 200 {
		test.Fatalf("Failed to create track via v3: %d", resp.StatusCode)
	}

	// GET via v2 should return language as comma-joined string
	v2Path := fmt.Sprintf("/v2/tracks?url=%s", escapedTrackUrl)
	v2Req := basicRequest(test, "GET", v2Path, "")
	resp2, _ := doRawRequest(test, v2Req)
	var track map[string]interface{}
	json.NewDecoder(resp2.Body).Decode(&track)
	tags := track["tags"].(map[string]interface{})

	// v2 should return language as a comma-joined string
	lang, ok := tags["language"].(string)
	if !ok {
		test.Errorf("Expected v2 language to be a string, got %T", tags["language"])
	}
	if lang != "en,fr" {
		test.Errorf("Expected v2 language to be 'en,fr', got '%s'", lang)
	}
}

func TestV3TrackByID(test *testing.T) {
	clearData()
	// Create track
	createReq := basicRequest(test, "PUT", "/v3/tracks?url="+url.QueryEscape("http://example.org/v3id/1"), `{"fingerprint": "v3id1", "duration": 150, "tags": {"title": "By ID Test", "composer": ["A"]}}`)
	doRawRequest(test, createReq)

	// GET by ID
	request := basicRequest(test, "GET", "/v3/tracks/1", "")
	resp, _ := doRawRequest(test, request)
	var track map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&track)

	assertEqual(test, "trackid", float64(1), track["trackid"].(float64))
	tags := track["tags"].(map[string]interface{})
	assertEqual(test, "title", "By ID Test", tags["title"].(string))
	composerArr := tags["composer"].([]interface{})
	if len(composerArr) != 1 {
		test.Errorf("Expected 1 composer, got %d", len(composerArr))
	}
}

func TestV3MethodNotAllowed(test *testing.T) {
	makeRequestWithUnallowedMethod(test, "/v3/tracks", "POST", []string{"GET", "PATCH"})
}

func TestV3PatchNonExistentTrack(test *testing.T) {
	clearData()
	request := basicRequest(test, "PATCH", "/v3/tracks?url="+url.QueryEscape("http://example.org/nonexistent"), `{"tags": {"title": "nope"}}`)
	resp, _ := doRawRequest(test, request)
	if resp.StatusCode != 404 {
		test.Errorf("Expected 404 for PATCH on non-existent track, got %d", resp.StatusCode)
	}
}

func TestV3DeleteTrack(test *testing.T) {
	clearData()
	// Create track
	createReq := basicRequest(test, "PUT", "/v3/tracks?url="+url.QueryEscape("http://example.org/v3del/1"), `{"fingerprint": "v3del1", "duration": 100, "tags": {"title": "Delete Me"}}`)
	doRawRequest(test, createReq)

	// Delete by ID
	makeRequest(test, "DELETE", "/v3/tracks/1", "", 204, "", false)

	// Verify gone
	getReq := basicRequest(test, "GET", "/v3/tracks/1", "")
	resp, _ := doRawRequest(test, getReq)
	if resp.StatusCode != 404 {
		test.Errorf("Expected 404 after delete, got %d", resp.StatusCode)
	}
}
