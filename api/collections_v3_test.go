package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"testing"
)

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
	// Create a track via v2
	trackurl := "http://example.org/v3coll/1"
	escapedTrackUrl := url.QueryEscape(trackurl)
	v2TrackPath := fmt.Sprintf("/v2/tracks?url=%s", escapedTrackUrl)
	makeRequest(test, "PUT", v2TrackPath, `{"fingerprint": "v3colltest1", "duration": 200, "tags": {"title": "Test Song", "artist": "Test Artist", "language": "en"}}`, 200, "", false)

	// Create a collection and add the track to it
	collPath := "/v3/collections/testcoll"
	makeRequest(test, "PUT", collPath, `{"name": "Test Collection", "icon": "🎵"}`, 200, "", false)
	makeRequest(test, "PUT", "/v3/collections/testcoll/1", "", 200, "Track In Collection\n", false)

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
	makeRequest(test, "PUT", "/v3/collections/first", `{"name": "First", "icon": "1️⃣"}`, 200, "", false)
	makeRequest(test, "PUT", "/v3/collections/second", `{"name": "Second", "icon": "2️⃣"}`, 200, "", false)

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
	makeRequest(test, "PUT", fmt.Sprintf("/v2/tracks?url=%s", escapedTrackUrl), `{"fingerprint": "v3collmem", "duration": 100}`, 200, "", false)
	makeRequest(test, "PUT", "/v3/collections/memtest", `{"name": "Membership Test", "icon": "🔗"}`, 200, "", false)

	// Track not in collection
	makeRequest(test, "GET", "/v3/collections/memtest/1", "", 404, "Track Not In Collection\n", false)

	// Add track to collection
	makeRequest(test, "PUT", "/v3/collections/memtest/1", "", 200, "Track In Collection\n", false)

	// Track in collection
	makeRequest(test, "GET", "/v3/collections/memtest/1", "", 200, "Track In Collection\n", false)

	// Remove track
	makeRequest(test, "DELETE", "/v3/collections/memtest/1", "", 200, "Track Not In Collection\n", false)

	// Track no longer in collection
	makeRequest(test, "GET", "/v3/collections/memtest/1", "", 404, "Track Not In Collection\n", false)
}

func TestV3DeleteCollection(test *testing.T) {
	clearData()
	makeRequest(test, "PUT", "/v3/collections/todelete", `{"name": "To Delete", "icon": "🗑️"}`, 200, "", false)
	makeRequest(test, "GET", "/v3/collections/todelete", "", 200, "", false)
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
	makeRequest(test, "PUT", fmt.Sprintf("/v2/tracks?url=%s", escapedTrackUrl), `{"fingerprint": "v3collrand", "duration": 180, "tags": {"title": "Random Track"}}`, 200, "", false)
	makeRequest(test, "PUT", "/v2/tracks/1/weighting", "5", 200, "", false)
	makeRequest(test, "PUT", "/v3/collections/randtest", `{"name": "Random Test", "icon": "🎲"}`, 200, "", false)
	makeRequest(test, "PUT", "/v3/collections/randtest/1", "", 200, "Track In Collection\n", false)

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
	makeRequest(test, "PUT", "/v3/collections/testendpoint", `{"name": "Endpoint Test", "icon": "🔍"}`, 200, "", false)
	makeRequest(test, "GET", "/v3/collections/testendpoint/unknown", "", 404, `{"error":"Collection Endpoint Not Found","code":"not_found"}`, true)
}

func TestV3CollectionTrackNotFound(test *testing.T) {
	clearData()
	makeRequest(test, "PUT", "/v3/collections/tracknotfound", `{"name": "Track Not Found Test", "icon": "❓"}`, 200, "", false)
	makeRequest(test, "GET", "/v3/collections/tracknotfound/999", "", 404, `{"error":"Track Not Found","code":"not_found"}`, true)
}

func TestV3CollectionRandomOnNonexistentCollection(test *testing.T) {
	clearData()
	makeRequest(test, "GET", "/v3/collections/nonexistent/random", "", 404, `{"error":"Collection Not Found","code":"not_found"}`, true)
}
