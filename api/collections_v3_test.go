package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"testing"
)

// setupRequest is a test helper that makes a request expecting a given status
// code but doesn't check the body — for setup steps where we only care about success.
func setupRequest(test *testing.T, method string, path string, requestBody string, expectedStatus int) {
	request := basicRequest(test, method, path, requestBody)
	resp, _ := doRawRequest(test, request)
	if resp.StatusCode != expectedStatus {
		test.Errorf("Setup %s %s: got status %d, expected %d", method, path, resp.StatusCode, expectedStatus)
	}
}

func TestV3CreateCollection(test *testing.T) {
	clearData()
	path := "/v3/collections/basic"
	inputJson := `{"name": "A Basic Collection", "icon": "🎷"}`
	outputJson := `{"slug":"basic","name": "A Basic Collection", "icon":"🎷", "tracks":[], "totalPages":0}`
	makeRequest(test, "GET", path, "", 404, `{"error":"Collection Not Found","code":"not_found"}`, true)
	makeRequest(test, "PUT", path, inputJson, 200, outputJson, true)
	assertEqual(test, "Loganne event type", "collectionCreated", lastLoganneType)
	makeRequest(test, "GET", path, "", 200, outputJson, true)
	restartServer()
	makeRequest(test, "GET", path, "", 200, outputJson, true)
	makeRequestWithUnallowedMethod(test, path, "POST", []string{"PUT", "GET", "DELETE"})
}

func TestV3GetCollectionTracksHaveStructuredTags(test *testing.T) {
	clearData()
	// Create a track via v3
	trackurl := "http://example.org/v3coll/1"
	escapedTrackUrl := url.QueryEscape(trackurl)
	trackPath := fmt.Sprintf("/v3/tracks?url=%s", escapedTrackUrl)
	setupRequest(test, "PUT", trackPath, `{"fingerprint": "v3colltest1", "duration": 200, "tags": {"title": [{"name": "Test Song"}], "artist": [{"name": "Test Artist"}], "language": [{"name": "en"}]}}`, 200)

	// Create a collection and add the track to it
	collPath := "/v3/collections/testcoll"
	setupRequest(test, "PUT", collPath, `{"name": "Test Collection", "icon": "🎵"}`, 200)
	makeRequest(test, "PUT", "/v3/collections/testcoll/1", "", 200, `{"inCollection":true}`, true)

	// GET the collection via v3 and verify tag format
	request := basicRequest(test, "GET", collPath, "")
	resp, _ := doRawRequest(test, request)
	var collection map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&collection)

	tracks := collection["tracks"].([]interface{})
	if len(tracks) != 1 {
		test.Fatalf("Expected 1 track, got %d", len(tracks))
	}
	track := tracks[0].(map[string]interface{})

	// Check tags are in v3 structured format
	tags := track["tags"].(map[string]interface{})
	titleArr, ok := tags["title"].([]interface{})
	if !ok {
		test.Fatalf("Expected title to be an array in v3 response, got %T", tags["title"])
	}
	if len(titleArr) != 1 {
		test.Errorf("Expected 1 title value, got %d", len(titleArr))
	}
	titleObj := titleArr[0].(map[string]interface{})
	assertEqual(test, "title name", "Test Song", titleObj["name"].(string))

	// Check "id" field exists, "trackid" does not
	if _, hasID := track["id"]; !hasID {
		test.Error("Expected 'id' field in v3 collection track")
	}
	if _, hasTrackid := track["trackid"]; hasTrackid {
		test.Error("Expected 'trackid' to NOT be present in v3 collection track")
	}
}

func TestV3GetAllCollections(test *testing.T) {
	clearData()
	// Create two collections
	setupRequest(test, "PUT", "/v3/collections/first", `{"name": "First", "icon": "1️⃣"}`, 200)
	setupRequest(test, "PUT", "/v3/collections/second", `{"name": "Second", "icon": "2️⃣"}`, 200)

	// GET all collections
	request := basicRequest(test, "GET", "/v3/collections", "")
	resp, _ := doRawRequest(test, request)
	var collections []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&collections)

	if len(collections) != 2 {
		test.Fatalf("Expected 2 collections, got %d", len(collections))
	}
	// Verify they have totalTracks and totalPages but no tracks array
	for _, c := range collections {
		if _, hasTotalTracks := c["totalTracks"]; !hasTotalTracks {
			test.Error("Expected 'totalTracks' in collection list item")
		}
		if _, hasTotalPages := c["totalPages"]; !hasTotalPages {
			test.Error("Expected 'totalPages' in collection list item")
		}
		if _, hasTracks := c["tracks"]; hasTracks {
			test.Error("Expected no 'tracks' array in collection list item")
		}
	}
}

func TestV3CollectionNotFound(test *testing.T) {
	clearData()
	makeRequest(test, "GET", "/v3/collections/nonexistent", "", 404, `{"error":"Collection Not Found","code":"not_found"}`, true)
}

func TestV3CollectionTrackMembership(test *testing.T) {
	clearData()
	// Create track and collection
	trackurl := "http://example.org/v3coll/membership"
	escapedTrackUrl := url.QueryEscape(trackurl)
	setupRequest(test, "PUT", fmt.Sprintf("/v3/tracks?url=%s", escapedTrackUrl), `{"fingerprint": "v3collmem", "duration": 100}`, 200)
	setupRequest(test, "PUT", "/v3/collections/memtest", `{"name": "Membership Test", "icon": "🔗"}`, 200)

	// Track not in collection — V3 uses structured JSON error
	makeRequest(test, "GET", "/v3/collections/memtest/1", "", 404, `{"error":"Track Not In Collection","code":"not_found"}`, true)

	// Add track to collection
	makeRequest(test, "PUT", "/v3/collections/memtest/1", "", 200, `{"inCollection":true}`, true)

	// Track in collection
	makeRequest(test, "GET", "/v3/collections/memtest/1", "", 200, `{"inCollection":true}`, true)

	// Remove track
	makeRequest(test, "DELETE", "/v3/collections/memtest/1", "", 200, `{"inCollection":false}`, true)

	// Track no longer in collection
	makeRequest(test, "GET", "/v3/collections/memtest/1", "", 404, `{"error":"Track Not In Collection","code":"not_found"}`, true)
}

func TestV3DeleteCollection(test *testing.T) {
	clearData()
	setupRequest(test, "PUT", "/v3/collections/todelete", `{"name": "To Delete", "icon": "🗑️"}`, 200)
	setupRequest(test, "GET", "/v3/collections/todelete", "", 200)
	makeRequest(test, "DELETE", "/v3/collections/todelete", "", 204, "", false)
	makeRequest(test, "GET", "/v3/collections/todelete", "", 404, `{"error":"Collection Not Found","code":"not_found"}`, true)
}

func TestV3CollectionPutNoBody(test *testing.T) {
	clearData()
	makeRequest(test, "PUT", "/v3/collections/nobody", "", 400, `{"error":"No Data Sent","code":"bad_request"}`, true)
}

func TestV3CollectionRandomEndpoint(test *testing.T) {
	clearData()
	// Create collection and add a weighted track
	trackurl := "http://example.org/v3coll/random"
	escapedTrackUrl := url.QueryEscape(trackurl)
	setupRequest(test, "PUT", fmt.Sprintf("/v3/tracks?url=%s", escapedTrackUrl), `{"fingerprint": "v3collrand", "duration": 180, "tags": {"title": [{"name": "Random Track"}]}}`, 200)
	setupRequest(test, "PUT", "/v3/tracks/1/weighting", "5", 200)
	setupRequest(test, "PUT", "/v3/collections/randtest", `{"name": "Random Test", "icon": "🎲"}`, 200)
	makeRequest(test, "PUT", "/v3/collections/randtest/1", "", 200, `{"inCollection":true}`, true)

	// GET random tracks
	request := basicRequest(test, "GET", "/v3/collections/randtest/random", "")
	resp, _ := doRawRequest(test, request)
	var collection map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&collection)

	tracks := collection["tracks"].([]interface{})
	if len(tracks) == 0 {
		test.Error("Expected random tracks to be returned")
	}

	// Verify first track has v3 tag format
	track := tracks[0].(map[string]interface{})
	tags := track["tags"].(map[string]interface{})
	titleArr, ok := tags["title"].([]interface{})
	if !ok {
		test.Fatalf("Expected title to be an array in v3 random response, got %T", tags["title"])
	}
	titleObj := titleArr[0].(map[string]interface{})
	assertEqual(test, "random track title name", "Random Track", titleObj["name"].(string))

	// Verify "id" not "trackid"
	if _, hasID := track["id"]; !hasID {
		test.Error("Expected 'id' in v3 random track")
	}
	if _, hasTrackid := track["trackid"]; hasTrackid {
		test.Error("Expected no 'trackid' in v3 random track")
	}
}

func TestV3CollectionEndpointNotFound(test *testing.T) {
	clearData()
	setupRequest(test, "PUT", "/v3/collections/testendpoint", `{"name": "Endpoint Test", "icon": "🔍"}`, 200)
	makeRequest(test, "GET", "/v3/collections/testendpoint/unknown", "", 404, `{"error":"Collection Endpoint Not Found","code":"not_found"}`, true)
}

func TestV3CollectionTrackNotFound(test *testing.T) {
	clearData()
	setupRequest(test, "PUT", "/v3/collections/tracknotfound", `{"name": "Track Not Found Test", "icon": "❓"}`, 200)
	makeRequest(test, "GET", "/v3/collections/tracknotfound/999", "", 404, `{"error":"Track Not Found","code":"not_found"}`, true)
}

func TestV3CollectionRandomMethodNotAllowed(test *testing.T) {
	clearData()
	setupRequest(test, "PUT", "/v3/collections/randmethod", `{"name": "Random Method Test", "icon": "🎲"}`, 200)
	makeRequestWithUnallowedMethod(test, "/v3/collections/randmethod/random", "POST", []string{"GET"})
	makeRequestWithUnallowedMethod(test, "/v3/collections/randmethod/random", "PUT", []string{"GET"})
	makeRequestWithUnallowedMethod(test, "/v3/collections/randmethod/random", "DELETE", []string{"GET"})
}

func TestV3CollectionRandomOnNonexistentCollection(test *testing.T) {
	clearData()
	makeRequest(test, "GET", "/v3/collections/nonexistent/random", "", 404, `{"error":"Collection Not Found","code":"not_found"}`, true)
}
