package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
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

func TestV3GetTrackReturnsStructuredTags(test *testing.T) {
	clearData()
	trackurl := "http://example.org/v3track/1"
	escapedTrackUrl := url.QueryEscape(trackurl)

	// Create track via v3 with structured tags
	v3Path := fmt.Sprintf("/v3/tracks?url=%s", escapedTrackUrl)
	v3Input := `{"fingerprint": "v3test1", "duration": 200, "tags": {"title": [{"name": "Test Song"}], "artist": [{"name": "Test Artist"}], "language": [{"name": "English", "uri": "https://eolas.l42.eu/metadata/language/en/"}]}}`
	request := basicRequest(test, "PUT", v3Path, v3Input)
	resp, _ := doRawRequest(test, request)
	if resp.StatusCode != 200 {
		test.Fatalf("Failed to create track via v3: %d", resp.StatusCode)
	}

	// GET via v3 should return all predicates as arrays of {name, uri} objects
	request2 := basicRequest(test, "GET", v3Path, "")
	resp2, _ := doRawRequest(test, request2)
	var track map[string]interface{}
	json.NewDecoder(resp2.Body).Decode(&track)
	tags := track["tags"].(map[string]interface{})

	// title is single-value -> array of one object
	titleArr, ok := tags["title"].([]interface{})
	if !ok {
		test.Fatalf("Expected title to be an array in v3 response, got %T", tags["title"])
	}
	if len(titleArr) != 1 {
		test.Errorf("Expected 1 title value, got %d", len(titleArr))
	}
	titleObj := titleArr[0].(map[string]interface{})
	assertEqual(test, "title name", "Test Song", titleObj["name"].(string))

	// language is single-value -> array of one object
	langArr, ok := tags["language"].([]interface{})
	if !ok {
		test.Fatalf("Expected language to be an array in v3 response, got %T", tags["language"])
	}
	if len(langArr) != 1 {
		test.Errorf("Expected 1 language value, got %d", len(langArr))
	}
	langObj := langArr[0].(map[string]interface{})
	assertEqual(test, "language name", "English", langObj["name"].(string))
}

func TestV3UsesIdNotTrackid(test *testing.T) {
	clearData()
	trackurl := "http://example.org/v3track/id-test"
	escapedTrackUrl := url.QueryEscape(trackurl)
	v3Path := fmt.Sprintf("/v3/tracks?url=%s", escapedTrackUrl)

	createReq := basicRequest(test, "PUT", v3Path, `{"fingerprint": "v3idtest", "duration": 150, "tags": {"title": [{"name": "ID Test"}]}}`)
	doRawRequest(test, createReq)

	// GET and check "id" field exists, "trackid" does not
	getReq := basicRequest(test, "GET", v3Path, "")
	resp, _ := doRawRequest(test, getReq)
	var raw map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&raw)

	if _, hasID := raw["id"]; !hasID {
		test.Error("Expected 'id' field in v3 response")
	}
	if _, hasTrackid := raw["trackid"]; hasTrackid {
		test.Error("Expected 'trackid' to NOT be present in v3 response")
	}
}

func TestV3NoDebugWeightingFields(test *testing.T) {
	clearData()
	// Create a track with weighting so random endpoint returns it
	createReq := basicRequest(test, "PUT", "/v3/tracks?url="+url.QueryEscape("http://example.org/v3track/weight-test"), `{"fingerprint": "v3wtest", "duration": 100, "tags": {"title": [{"name": "Weight Test"}]}}`)
	doRawRequest(test, createReq)
	// Set weighting via v3
	weightReq := basicRequest(test, "PUT", "/v3/tracks/1/weighting", "5")
	doRawRequest(test, weightReq)

	// GET single track
	getReq := basicRequest(test, "GET", "/v3/tracks/1", "")
	resp, _ := doRawRequest(test, getReq)
	var raw map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&raw)

	if _, has := raw["_random_weighting"]; has {
		test.Error("v3 response should not contain _random_weighting")
	}
	if _, has := raw["_cum_weighting"]; has {
		test.Error("v3 response should not contain _cum_weighting")
	}
}

func TestV3PutTrackWithStructuredTags(test *testing.T) {
	clearData()
	trackurl := "http://example.org/v3track/2"
	escapedTrackUrl := url.QueryEscape(trackurl)
	v3Path := fmt.Sprintf("/v3/tracks?url=%s", escapedTrackUrl)

	inputJSON := `{"fingerprint": "v3test2", "duration": 300, "tags": {"title": [{"name": "Multi Test"}], "language": [{"name": "English", "uri": "https://eolas.l42.eu/metadata/language/en/"}, {"name": "French", "uri": "https://eolas.l42.eu/metadata/language/fr/"}], "composer": [{"name": "A"}, {"name": "B"}]}}`
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

	// Check title is array with 1 object
	titleArr := tags["title"].([]interface{})
	if len(titleArr) != 1 {
		test.Errorf("Expected 1 title, got %d", len(titleArr))
	}
	assertEqual(test, "title name", "Multi Test", titleArr[0].(map[string]interface{})["name"].(string))

	// Check language is array with 2 objects, with URIs
	langArr := tags["language"].([]interface{})
	if len(langArr) != 2 {
		test.Errorf("Expected 2 language values, got %d", len(langArr))
	}
	lang0 := langArr[0].(map[string]interface{})
	assertEqual(test, "language[0].name", "English", lang0["name"].(string))
	assertEqual(test, "language[0].uri", "https://eolas.l42.eu/metadata/language/en/", lang0["uri"].(string))

	// Check composer is array with 2 objects
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
	createReq := basicRequest(test, "PUT", v3Path, `{"fingerprint": "v3test3", "duration": 200, "tags": {"title": [{"name": "Original"}], "language": [{"name": "English", "uri": "https://eolas.l42.eu/metadata/language/en/"}]}}`)
	resp, _ := doRawRequest(test, createReq)
	if resp.StatusCode != 200 {
		test.Fatalf("Failed to create track: %d", resp.StatusCode)
	}

	// PATCH to update language
	patchReq := basicRequest(test, "PATCH", v3Path, `{"tags": {"language": [{"name": "English", "uri": "https://eolas.l42.eu/metadata/language/en/"}, {"name": "German", "uri": "https://eolas.l42.eu/metadata/language/de/"}, {"name": "French", "uri": "https://eolas.l42.eu/metadata/language/fr/"}]}}`)
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

func TestV3RejectsStringForPredicate(test *testing.T) {
	clearData()
	trackurl := "http://example.org/v3track/4"
	escapedTrackUrl := url.QueryEscape(trackurl)
	v3Path := fmt.Sprintf("/v3/tracks?url=%s", escapedTrackUrl)

	// Should reject a plain string for any predicate (v3 requires arrays of objects)
	request := basicRequest(test, "PUT", v3Path, `{"fingerprint": "v3test4", "duration": 200, "tags": {"title": "just a string"}}`)
	resp, _ := doRawRequest(test, request)
	if resp.StatusCode != 400 {
		test.Errorf("Expected 400 for string predicate value, got %d", resp.StatusCode)
	}
}

func TestV3RejectsMultipleValuesForSingleValuePredicate(test *testing.T) {
	clearData()
	trackurl := "http://example.org/v3track/5"
	escapedTrackUrl := url.QueryEscape(trackurl)
	v3Path := fmt.Sprintf("/v3/tracks?url=%s", escapedTrackUrl)

	// Should reject multiple values for a single-value predicate
	request := basicRequest(test, "PUT", v3Path, `{"fingerprint": "v3test5", "duration": 200, "tags": {"title": [{"name": "a"}, {"name": "b"}]}}`)
	resp, _ := doRawRequest(test, request)
	if resp.StatusCode != 400 {
		test.Errorf("Expected 400 for multiple values on single-value predicate, got %d", resp.StatusCode)
	}
}

func TestV3GetMultipleTracks(test *testing.T) {
	clearData()
	// Create two tracks via v3
	req1 := basicRequest(test, "PUT", "/v3/tracks?url="+url.QueryEscape("http://example.org/v3multi/1"), `{"fingerprint": "v3m1", "duration": 100, "tags": {"title": [{"name": "Track A"}], "language": [{"name": "English", "uri": "https://eolas.l42.eu/metadata/language/en/"}]}}`)
	doRawRequest(test, req1)
	req2 := basicRequest(test, "PUT", "/v3/tracks?url="+url.QueryEscape("http://example.org/v3multi/2"), `{"fingerprint": "v3m2", "duration": 200, "tags": {"title": [{"name": "Track B"}], "language": [{"name": "French", "uri": "https://eolas.l42.eu/metadata/language/fr/"}, {"name": "German", "uri": "https://eolas.l42.eu/metadata/language/de/"}]}}`)
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

	// Verify pagination fields
	if _, has := result["page"]; !has {
		test.Error("Expected 'page' field in search result")
	}
	if _, has := result["totalTracks"]; !has {
		test.Error("Expected 'totalTracks' field in search result")
	}

	// Verify that tracks in list use structured v3 format
	for _, trackRaw := range tracks {
		track := trackRaw.(map[string]interface{})
		tags := track["tags"].(map[string]interface{})
		if lang, ok := tags["language"]; ok {
			langArr, isArr := lang.([]interface{})
			if !isArr {
				test.Errorf("Expected language in list track to be array, got %T", lang)
			} else if len(langArr) > 0 {
				// Each element should be an object with "name"
				obj, isObj := langArr[0].(map[string]interface{})
				if !isObj {
					test.Errorf("Expected language array element to be object, got %T", langArr[0])
				} else if _, hasName := obj["name"]; !hasName {
					test.Error("Expected language object to have 'name' field")
				}
			}
		}
		// Verify uses "id" not "trackid"
		if _, hasID := track["id"]; !hasID {
			test.Error("Expected 'id' field in track list item")
		}
		if _, hasTrackid := track["trackid"]; hasTrackid {
			test.Error("Expected 'trackid' to NOT be present in track list item")
		}
	}
}

func TestV3PaginationFields(test *testing.T) {
	clearData()
	// Create one track
	req := basicRequest(test, "PUT", "/v3/tracks?url="+url.QueryEscape("http://example.org/v3page/1"), `{"fingerprint": "v3p1", "duration": 100, "tags": {"title": [{"name": "Page Test"}]}}`)
	doRawRequest(test, req)

	request := basicRequest(test, "GET", "/v3/tracks?page=1", "")
	resp, _ := doRawRequest(test, request)
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	assertEqual(test, "page", float64(1), result["page"].(float64))
	assertEqual(test, "totalTracks", float64(1), result["totalTracks"].(float64))
	assertEqual(test, "totalPages", float64(1), result["totalPages"].(float64))
}


func TestV3TrackByID(test *testing.T) {
	clearData()
	// Create track
	createReq := basicRequest(test, "PUT", "/v3/tracks?url="+url.QueryEscape("http://example.org/v3id/1"), `{"fingerprint": "v3id1", "duration": 150, "tags": {"title": [{"name": "By ID Test"}], "composer": [{"name": "A"}]}}`)
	doRawRequest(test, createReq)

	// GET by ID
	request := basicRequest(test, "GET", "/v3/tracks/1", "")
	resp, _ := doRawRequest(test, request)
	var track map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&track)

	assertEqual(test, "id", float64(1), track["id"].(float64))
	tags := track["tags"].(map[string]interface{})
	titleArr := tags["title"].([]interface{})
	if len(titleArr) != 1 {
		test.Errorf("Expected 1 title, got %d", len(titleArr))
	}
	assertEqual(test, "title name", "By ID Test", titleArr[0].(map[string]interface{})["name"].(string))
	composerArr := tags["composer"].([]interface{})
	if len(composerArr) != 1 {
		test.Errorf("Expected 1 composer, got %d", len(composerArr))
	}
}

func TestV3StructuredErrors(test *testing.T) {
	clearData()
	// PATCH non-existent track should return structured JSON error
	request := basicRequest(test, "PATCH", "/v3/tracks?url="+url.QueryEscape("http://example.org/nonexistent"), `{"tags": {"title": [{"name": "nope"}]}}`)
	resp, _ := doRawRequest(test, request)
	if resp.StatusCode != 404 {
		test.Errorf("Expected 404, got %d", resp.StatusCode)
	}
	var errResp V3Error
	json.NewDecoder(resp.Body).Decode(&errResp)
	if errResp.Code != "not_found" {
		test.Errorf("Expected error code 'not_found', got '%s'", errResp.Code)
	}
	if errResp.Error != "Track Not Found" {
		test.Errorf("Expected error message 'Track Not Found', got '%s'", errResp.Error)
	}
}

func TestV3MethodNotAllowed(test *testing.T) {
	makeRequestWithUnallowedMethod(test, "/v3/tracks", "POST", []string{"GET", "PATCH"})
}

func TestV3PatchNonExistentTrack(test *testing.T) {
	clearData()
	request := basicRequest(test, "PATCH", "/v3/tracks?url="+url.QueryEscape("http://example.org/nonexistent"), `{"tags": {"title": [{"name": "nope"}]}}`)
	resp, _ := doRawRequest(test, request)
	if resp.StatusCode != 404 {
		test.Errorf("Expected 404 for PATCH on non-existent track, got %d", resp.StatusCode)
	}
}

func TestV3DeleteTrack(test *testing.T) {
	clearData()
	// Create track
	createReq := basicRequest(test, "PUT", "/v3/tracks?url="+url.QueryEscape("http://example.org/v3del/1"), `{"fingerprint": "v3del1", "duration": 100, "tags": {"title": [{"name": "Delete Me"}]}}`)
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

func TestV3GetTrackRDFTurtle(test *testing.T) {
	clearData()
	trackurl := "http://example.org/v3rdf/turtle"
	escapedTrackUrl := url.QueryEscape(trackurl)
	v3Path := fmt.Sprintf("/v3/tracks?url=%s", escapedTrackUrl)

	// Create track
	inputJSON := `{"fingerprint": "v3rdf1", "duration": 180, "tags": {"title": [{"name": "RDF Test Track"}], "artist": [{"name": "Test Artist"}]}}`
	createReq := basicRequest(test, "PUT", v3Path, inputJSON)
	resp, _ := doRawRequest(test, createReq)
	if resp.StatusCode != 200 {
		test.Fatalf("Failed to create track: %d", resp.StatusCode)
	}

	// GET with text/turtle Accept header should return RDF
	getReq := basicRequest(test, "GET", v3Path, "")
	getReq.Header.Set("Accept", "text/turtle")
	resp2, _ := doRawRequest(test, getReq)
	if resp2.StatusCode != 200 {
		test.Errorf("Expected 200, got %d", resp2.StatusCode)
	}
	ct := resp2.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/turtle") {
		test.Errorf("Expected Content-Type text/turtle, got %s", ct)
	}
}

func TestV3GetTrackRDFJsonLD(test *testing.T) {
	clearData()
	trackurl := "http://example.org/v3rdf/jsonld"
	escapedTrackUrl := url.QueryEscape(trackurl)
	v3Path := fmt.Sprintf("/v3/tracks?url=%s", escapedTrackUrl)

	// Create track
	inputJSON := `{"fingerprint": "v3rdf2", "duration": 210, "tags": {"title": [{"name": "JSON-LD Track"}]}}`
	createReq := basicRequest(test, "PUT", v3Path, inputJSON)
	resp, _ := doRawRequest(test, createReq)
	if resp.StatusCode != 200 {
		test.Fatalf("Failed to create track: %d", resp.StatusCode)
	}

	// GET with application/ld+json Accept header should return RDF
	getReq := basicRequest(test, "GET", v3Path, "")
	getReq.Header.Set("Accept", "application/ld+json")
	resp2, _ := doRawRequest(test, getReq)
	if resp2.StatusCode != 200 {
		test.Errorf("Expected 200, got %d", resp2.StatusCode)
	}
	ct := resp2.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/ld+json") {
		test.Errorf("Expected Content-Type application/ld+json, got %s", ct)
	}
}

func TestV3GetTrackDefaultsToJSON(test *testing.T) {
	clearData()
	trackurl := "http://example.org/v3rdf/json-default"
	escapedTrackUrl := url.QueryEscape(trackurl)
	v3Path := fmt.Sprintf("/v3/tracks?url=%s", escapedTrackUrl)

	// Create track
	inputJSON := `{"fingerprint": "v3rdf3", "duration": 150, "tags": {"title": [{"name": "JSON Default Track"}]}}`
	createReq := basicRequest(test, "PUT", v3Path, inputJSON)
	resp, _ := doRawRequest(test, createReq)
	if resp.StatusCode != 200 {
		test.Fatalf("Failed to create track: %d", resp.StatusCode)
	}

	// GET with no Accept header should return JSON
	getReq := basicRequest(test, "GET", v3Path, "")
	resp2, _ := doRawRequest(test, getReq)
	if resp2.StatusCode != 200 {
		test.Errorf("Expected 200, got %d", resp2.StatusCode)
	}
	ct := resp2.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		test.Errorf("Expected Content-Type application/json, got %s", ct)
	}
}

func TestV3URIStoredAndReturned(test *testing.T) {
	clearData()
	trackurl := "http://example.org/v3track/uri-test"
	escapedTrackUrl := url.QueryEscape(trackurl)
	v3Path := fmt.Sprintf("/v3/tracks?url=%s", escapedTrackUrl)

	// Create track with URI on a tag
	inputJSON := `{"fingerprint": "v3uri1", "duration": 200, "tags": {"title": [{"name": "URI Test"}], "language": [{"name": "English", "uri": "https://eolas.l42.eu/metadata/language/en/"}]}}`
	createReq := basicRequest(test, "PUT", v3Path, inputJSON)
	resp, _ := doRawRequest(test, createReq)
	if resp.StatusCode != 200 {
		test.Fatalf("Failed to create track: %d", resp.StatusCode)
	}

	// GET and verify URI is returned
	getReq := basicRequest(test, "GET", v3Path, "")
	resp2, _ := doRawRequest(test, getReq)
	var track map[string]interface{}
	json.NewDecoder(resp2.Body).Decode(&track)
	tags := track["tags"].(map[string]interface{})
	langArr := tags["language"].([]interface{})
	langObj := langArr[0].(map[string]interface{})
	assertEqual(test, "language uri", "https://eolas.l42.eu/metadata/language/en/", langObj["uri"].(string))

	// title should not have uri
	titleArr := tags["title"].([]interface{})
	titleObj := titleArr[0].(map[string]interface{})
	if _, hasURI := titleObj["uri"]; hasURI {
		test.Error("Expected title to not have 'uri' field when empty")
	}
}

func TestV3PatchEmptyArrayClearsField(test *testing.T) {
	clearData()
	trackurl := "http://example.org/v3track/clear-test"
	escapedTrackUrl := url.QueryEscape(trackurl)
	v3Path := fmt.Sprintf("/v3/tracks?url=%s", escapedTrackUrl)

	// Create track with language and title tags
	createReq := basicRequest(test, "PUT", v3Path, `{"fingerprint": "v3clear1", "duration": 200, "tags": {"title": [{"name": "Clear Test"}], "language": [{"name": "English", "uri": "https://eolas.l42.eu/metadata/language/en/"}]}}`)
	resp, _ := doRawRequest(test, createReq)
	if resp.StatusCode != 200 {
		test.Fatalf("Failed to create track: %d", resp.StatusCode)
	}

	// PATCH with empty array for language — should clear it
	patchReq := basicRequest(test, "PATCH", v3Path, `{"tags": {"language": []}}`)
	resp2, _ := doRawRequest(test, patchReq)
	if resp2.StatusCode != 200 {
		test.Fatalf("Failed to PATCH track: %d", resp2.StatusCode)
	}

	// Verify language is cleared
	getReq := basicRequest(test, "GET", v3Path, "")
	resp3, _ := doRawRequest(test, getReq)
	var track map[string]interface{}
	json.NewDecoder(resp3.Body).Decode(&track)
	tags := track["tags"].(map[string]interface{})
	if _, hasLang := tags["language"]; hasLang {
		test.Error("Expected 'language' to be absent after PATCH with empty array")
	}
	// title should be unaffected
	titleArr2, ok := tags["title"].([]interface{})
	if !ok {
		test.Fatal("Expected 'title' to still be present after PATCH")
	}
	if len(titleArr2) != 1 {
		test.Errorf("Expected title to still have 1 value, got %d", len(titleArr2))
	}
}

func TestV3PatchMultiplePredicatesAtomic(test *testing.T) {
	clearData()
	trackurl := "http://example.org/v3track/atomic-test"
	escapedTrackUrl := url.QueryEscape(trackurl)
	v3Path := fmt.Sprintf("/v3/tracks?url=%s", escapedTrackUrl)

	// Create track with title and language
	createReq := basicRequest(test, "PUT", v3Path, `{"fingerprint": "v3atomic1", "duration": 300, "tags": {"title": [{"name": "Atomic Test"}], "language": [{"name": "English", "uri": "https://eolas.l42.eu/metadata/language/en/"}]}}`)
	resp, _ := doRawRequest(test, createReq)
	if resp.StatusCode != 200 {
		test.Fatalf("Failed to create track: %d", resp.StatusCode)
	}

	// PATCH updating both title and language in a single request
	patchReq := basicRequest(test, "PATCH", v3Path, `{"tags": {"title": [{"name": "Updated Title"}], "language": [{"name": "Irish", "uri": "https://eolas.l42.eu/metadata/language/ga/"}]}}`)
	resp2, _ := doRawRequest(test, patchReq)
	if resp2.StatusCode != 200 {
		test.Fatalf("Failed to PATCH track: %d", resp2.StatusCode)
	}

	// Verify both predicates were updated
	getReq := basicRequest(test, "GET", v3Path, "")
	resp3, _ := doRawRequest(test, getReq)
	var track map[string]interface{}
	json.NewDecoder(resp3.Body).Decode(&track)
	tags := track["tags"].(map[string]interface{})

	titleArr, ok := tags["title"].([]interface{})
	if !ok || len(titleArr) != 1 {
		test.Fatalf("Expected title to have 1 value, got %v", tags["title"])
	}
	if titleArr[0].(map[string]interface{})["name"] != "Updated Title" {
		test.Errorf("Expected title 'Updated Title', got %v", titleArr[0])
	}

	langArr, ok := tags["language"].([]interface{})
	if !ok || len(langArr) != 1 {
		test.Fatalf("Expected language to have 1 value, got %v", tags["language"])
	}
	if langArr[0].(map[string]interface{})["name"] != "Irish" {
		test.Errorf("Expected language 'Irish', got %v", langArr[0])
	}
}

// TestV3RejectsEmptyNameAndURI checks that a tag value with both name and uri empty is rejected.
func TestV3RejectsEmptyNameAndURI(test *testing.T) {
	clearData()
	trackurl := "http://example.org/v3track/empty-tag"
	v3Path := fmt.Sprintf("/v3/tracks?url=%s", url.QueryEscape(trackurl))

	// PUT with {name: "", uri: ""} — must be rejected.
	req := basicRequest(test, "PUT", v3Path, `{"fingerprint":"v3empty1","duration":100,"tags":{"comment":[{"name":"","uri":""}]}}`)
	resp, _ := doRawRequest(test, req)
	if resp.StatusCode != 400 {
		test.Errorf("Expected 400 for empty name+uri tag value, got %d", resp.StatusCode)
	}
	var errResp V3Error
	json.NewDecoder(resp.Body).Decode(&errResp)
	if errResp.Code != "invalid_tag_value" {
		test.Errorf("Expected code 'invalid_tag_value', got %q", errResp.Code)
	}
	if errResp.Predicate != "comment" {
		test.Errorf("Expected predicate 'comment', got %q", errResp.Predicate)
	}
}

// TestV3RejectsEmptyNameTag checks that a tag value with empty name (and no uri) is rejected.
func TestV3RejectsEmptyNameTag(test *testing.T) {
	clearData()
	trackurl := "http://example.org/v3track/empty-name"
	v3Path := fmt.Sprintf("/v3/tracks?url=%s", url.QueryEscape(trackurl))

	req := basicRequest(test, "PUT", v3Path, `{"fingerprint":"v3empty2","duration":100,"tags":{"title":[{"name":""}]}}`)
	resp, _ := doRawRequest(test, req)
	if resp.StatusCode != 400 {
		test.Errorf("Expected 400 for empty name tag value, got %d", resp.StatusCode)
	}
	var errResp V3Error
	json.NewDecoder(resp.Body).Decode(&errResp)
	if errResp.Code != "invalid_tag_value" {
		test.Errorf("Expected code 'invalid_tag_value', got %q", errResp.Code)
	}
	if errResp.Predicate != "title" {
		test.Errorf("Expected predicate 'title', got %q", errResp.Predicate)
	}
}

// TestV3RejectsNullTagArray checks that a null array for a predicate is rejected (use [] to clear).
func TestV3RejectsNullTagArray(test *testing.T) {
	clearData()
	trackurl := "http://example.org/v3track/null-tag"
	v3Path := fmt.Sprintf("/v3/tracks?url=%s", url.QueryEscape(trackurl))

	req := basicRequest(test, "PUT", v3Path, `{"fingerprint":"v3null1","duration":100,"tags":{"comment":null}}`)
	resp, _ := doRawRequest(test, req)
	if resp.StatusCode != 400 {
		test.Errorf("Expected 400 for null tag array, got %d", resp.StatusCode)
	}
	var errResp V3Error
	json.NewDecoder(resp.Body).Decode(&errResp)
	if errResp.Code != "invalid_tag_value" {
		test.Errorf("Expected code 'invalid_tag_value', got %q", errResp.Code)
	}
}

// TestV3AcceptsURIOnlyTagValue checks that a value with empty name but non-empty uri is still valid.
func TestV3AcceptsURIOnlyTagValue(test *testing.T) {
	clearData()
	// Create album first so the URI resolves.
	setupRequest(test, "POST", "/v3/albums", `{"name":"Abbey Road"}`, 201)

	trackurl := "http://example.org/v3track/uri-only"
	v3Path := fmt.Sprintf("/v3/tracks?url=%s", url.QueryEscape(trackurl))

	req := basicRequest(test, "PUT", v3Path, `{"fingerprint":"v3urionly1","duration":100,"tags":{"album":[{"uri":"/albums/1"}]}}`)
	resp, _ := doRawRequest(test, req)
	if resp.StatusCode != 200 {
		test.Errorf("Expected 200 for URI-only album tag, got %d", resp.StatusCode)
	}
}

// TestV3AcceptsClearWithEmptyArray checks that an empty array for a predicate is accepted (clears it).
func TestV3AcceptsClearWithEmptyArray(test *testing.T) {
	clearData()
	trackurl := "http://example.org/v3track/clear-empty-arr"
	v3Path := fmt.Sprintf("/v3/tracks?url=%s", url.QueryEscape(trackurl))

	setupRequest(test, "PUT", v3Path, `{"fingerprint":"v3clr1","duration":100,"tags":{"title":[{"name":"To Clear"}]}}`, 200)

	// PATCH with [] to clear the predicate — must succeed.
	req := basicRequest(test, "PATCH", v3Path, `{"tags":{"title":[]}}`)
	resp, _ := doRawRequest(test, req)
	if resp.StatusCode != 200 {
		test.Errorf("Expected 200 for empty-array predicate clear, got %d", resp.StatusCode)
	}
}

// TestV3RejectsEmptyTagOnPatch checks validation also fires on PATCH.
func TestV3RejectsEmptyTagOnPatch(test *testing.T) {
	clearData()
	trackurl := "http://example.org/v3track/empty-patch"
	v3Path := fmt.Sprintf("/v3/tracks?url=%s", url.QueryEscape(trackurl))
	setupRequest(test, "PUT", v3Path, `{"fingerprint":"v3epatch1","duration":100,"tags":{"title":[{"name":"Original"}]}}`, 200)

	req := basicRequest(test, "PATCH", v3Path, `{"tags":{"title":[{"name":""}]}}`)
	resp, _ := doRawRequest(test, req)
	if resp.StatusCode != 400 {
		test.Errorf("Expected 400 for empty name on PATCH, got %d", resp.StatusCode)
	}
}

// TestV3PutSingleTrackSetsTrackActionHeaderTrackAdded checks that PUTting a new
// track returns Track-Action: trackAdded.
func TestV3PutSingleTrackSetsTrackActionHeaderTrackAdded(test *testing.T) {
	clearData()
	v3Path := "/v3/tracks?url=" + url.QueryEscape("http://example.org/v3track/ta-new")
	req := basicRequest(test, "PUT", v3Path, `{"fingerprint":"ta1","duration":100,"tags":{"title":[{"name":"New Track"}]}}`)
	resp, _ := doRawRequest(test, req)
	if resp.StatusCode != 200 {
		test.Fatalf("Expected 200, got %d", resp.StatusCode)
	}
	assertEqual(test, "Track-Action header", "trackAdded", resp.Header.Get("Track-Action"))
}

// TestV3PutSingleTrackSetsTrackActionHeaderTrackUpdated checks that PUTting an
// existing track with a changed duration returns Track-Action: trackUpdated.
func TestV3PutSingleTrackSetsTrackActionHeaderTrackUpdated(test *testing.T) {
	clearData()
	v3Path := "/v3/tracks?url=" + url.QueryEscape("http://example.org/v3track/ta-update")
	setupRequest(test, "PUT", v3Path, `{"fingerprint":"ta2","duration":100}`, 200)

	req := basicRequest(test, "PUT", v3Path, `{"fingerprint":"ta2","duration":200}`)
	resp, _ := doRawRequest(test, req)
	if resp.StatusCode != 200 {
		test.Fatalf("Expected 200, got %d", resp.StatusCode)
	}
	assertEqual(test, "Track-Action header", "trackUpdated", resp.Header.Get("Track-Action"))
}

// TestV3PutSingleTrackSetsTrackActionHeaderNoChange checks that PUTting identical
// data returns Track-Action: noChange.
func TestV3PutSingleTrackSetsTrackActionHeaderNoChange(test *testing.T) {
	clearData()
	v3Path := "/v3/tracks?url=" + url.QueryEscape("http://example.org/v3track/ta-noop")
	setupRequest(test, "PUT", v3Path, `{"fingerprint":"ta3","duration":100}`, 200)

	req := basicRequest(test, "PUT", v3Path, `{"fingerprint":"ta3","duration":100}`)
	resp, _ := doRawRequest(test, req)
	if resp.StatusCode != 200 {
		test.Fatalf("Expected 200, got %d", resp.StatusCode)
	}
	assertEqual(test, "Track-Action header", "noChange", resp.Header.Get("Track-Action"))
}

// TestV3PatchSingleTrackSetsTrackActionHeader checks that PATCH sets Track-Action.
func TestV3PatchSingleTrackSetsTrackActionHeader(test *testing.T) {
	clearData()
	v3Path := "/v3/tracks?url=" + url.QueryEscape("http://example.org/v3track/ta-patch")
	setupRequest(test, "PUT", v3Path, `{"fingerprint":"ta4","duration":100}`, 200)

	req := basicRequest(test, "PATCH", v3Path, `{"duration":150}`)
	resp, _ := doRawRequest(test, req)
	if resp.StatusCode != 200 {
		test.Fatalf("Expected 200, got %d", resp.StatusCode)
	}
	assertEqual(test, "Track-Action header", "trackUpdated", resp.Header.Get("Track-Action"))
}

// TestV3PutTagOnlyChangeSetsTrackActionHeader documents that tag-only PUTs return
// noChange on the scalar-fields path (PR #198 strips tags from internalTrack to
// prevent the old URI-unaware write path from running; tag-only changes do not
// drive updateNeeded).
// TestV3PutTagOnlyChangeSetsTrackActionHeader checks that a PUT where only tags
// change fires trackUpdated and returns Track-Action: trackUpdated.
func TestV3PutTagOnlyChangeSetsTrackActionHeader(test *testing.T) {
	clearData()
	v3Path := "/v3/tracks?url=" + url.QueryEscape("http://example.org/v3track/ta-tagonly")
	setupRequest(test, "PUT", v3Path, `{"fingerprint":"ta5","duration":100,"tags":{"title":[{"name":"Old Title"}]}}`, 200)
	loganneRequestCount = 0

	req := basicRequest(test, "PUT", v3Path, `{"fingerprint":"ta5","duration":100,"tags":{"title":[{"name":"New Title"}]}}`)
	resp, _ := doRawRequest(test, req)
	if resp.StatusCode != 200 {
		test.Fatalf("Expected 200, got %d", resp.StatusCode)
	}
	assertEqual(test, "Track-Action header", "trackUpdated", resp.Header.Get("Track-Action"))
	assertEqual(test, "Loganne event type", "trackUpdated", lastLoganneType)
	assertEqual(test, "Loganne request count", 1, loganneRequestCount)
}

// TestV3PutTagOnlyNoChangeIsNoOp checks that a PUT with identical tags does not
// fire Loganne and returns Track-Action: noChange.
func TestV3PutTagOnlyNoChangeIsNoOp(test *testing.T) {
	clearData()
	v3Path := "/v3/tracks?url=" + url.QueryEscape("http://example.org/v3track/ta-tagonly-noop")
	setupRequest(test, "PUT", v3Path, `{"fingerprint":"ta6","duration":100,"tags":{"title":[{"name":"Same Title"}]}}`, 200)
	loganneRequestCount = 0

	req := basicRequest(test, "PUT", v3Path, `{"fingerprint":"ta6","duration":100,"tags":{"title":[{"name":"Same Title"}]}}`)
	resp, _ := doRawRequest(test, req)
	if resp.StatusCode != 200 {
		test.Fatalf("Expected 200, got %d", resp.StatusCode)
	}
	assertEqual(test, "Track-Action header", "noChange", resp.Header.Get("Track-Action"))
	assertEqual(test, "Loganne request count", 0, loganneRequestCount)
}
