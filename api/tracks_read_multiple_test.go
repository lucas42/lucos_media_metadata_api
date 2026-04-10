package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

/**
 * Checks whether a list of all tracks can be returned
 */
func TestGetAllTracks(test *testing.T) {
	clearData()
	track1url := "http://example.org/track/1256"
	track2url := "http://example.org/track/abcdef"
	path1 := fmt.Sprintf("/v3/tracks?url=%s", url.QueryEscape(track1url))
	path2 := fmt.Sprintf("/v3/tracks?url=%s", url.QueryEscape(track2url))
	pathall := "/v3/tracks"

	setupRequest(test, "PUT", path1, `{"fingerprint": "aoecu1234", "duration": 300}`, 200)
	setupRequest(test, "PUT", path2, `{"fingerprint": "blahdebo", "duration": 150}`, 200)

	request := basicRequest(test, "GET", pathall, "")
	resp, _ := doRawRequest(test, request)
	var result SearchResultV3
	json.NewDecoder(resp.Body).Decode(&result)

	if len(result.Tracks) != 2 {
		test.Errorf("Expected 2 tracks, got %d", len(result.Tracks))
	}
	if result.TotalPages != 1 {
		test.Errorf("Expected totalPages 1, got %d", result.TotalPages)
	}
	if result.TotalTracks != 2 {
		test.Errorf("Expected totalTracks 2, got %d", result.TotalTracks)
	}
	if result.Page != 1 {
		test.Errorf("Expected page 1, got %d", result.Page)
	}

	// Verify track fields use "id" not "trackid"
	for _, track := range result.Tracks {
		if track.ID == 0 {
			test.Error("Expected non-zero id in track")
		}
	}

	makeRequestWithUnallowedMethod(test, pathall, "PUT", []string{"GET", "PATCH"})
}

/**
 * Checks random tracks endpoint returns up to 20 tracks from a pool
 */
func TestRandomTracks(test *testing.T) {
	clearData()
	path := "/v3/tracks/random"

	request := basicRequest(test, "GET", path, "")
	resp, _ := doRawRequest(test, request)
	var empty SearchResultV3
	json.NewDecoder(resp.Body).Decode(&empty)
	if len(empty.Tracks) != 0 {
		test.Errorf("Expected 0 tracks on empty library, got %d", len(empty.Tracks))
	}

	// Create 40 tracks with weightings
	for i := 1; i <= 40; i++ {
		id := strconv.Itoa(i)
		escapedURL := url.QueryEscape("http://example.org/track/id" + id)
		setupRequest(test, "PUT", fmt.Sprintf("/v3/tracks?url=%s", escapedURL), `{"fingerprint": "abcde`+id+`", "duration": 350}`, 200)
		setupRequest(test, "PUT", "/v3/tracks/"+id+"/weighting", "4.3", 200)
	}

	request2 := basicRequest(test, "GET", path, "")
	response, err := http.DefaultClient.Do(request2)
	if err != nil {
		test.Error(err)
		return
	}
	if response.StatusCode != 200 {
		test.Errorf("Got response code %d for %s", response.StatusCode, path)
	}
	var result SearchResultV3
	json.NewDecoder(response.Body).Decode(&result)
	if len(result.Tracks) != 20 {
		test.Errorf("Expected 20 random tracks, got %d", len(result.Tracks))
	}
	if result.TotalPages != 1 {
		test.Errorf("Expected totalPages 1, got %d", result.TotalPages)
	}
	if !strings.Contains(response.Header.Get("Cache-Control"), "no-cache") {
		test.Error("Random track list should be no-cache")
	}
	if !strings.Contains(response.Header.Get("Cache-Control"), "max-age=0") {
		test.Error("Random track should have max-age=0")
	}
}

/**
 * Checks that after deleting most tracks, random only returns the remaining one
 */
func TestRandomTracksDealsWithDeletes(test *testing.T) {
	clearData()

	// Create 40 tracks with weightings
	for i := 1; i <= 40; i++ {
		id := strconv.Itoa(i)
		escapedURL := url.QueryEscape("http://example.org/track/id" + id)
		setupRequest(test, "PUT", fmt.Sprintf("/v3/tracks?url=%s", escapedURL), `{"fingerprint": "abcde`+id+`", "duration": 350}`, 200)
		setupRequest(test, "PUT", "/v3/tracks/"+id+"/weighting", "4.3", 200)
	}

	// Delete 38 of them
	for i := 1; i <= 38; i++ {
		setupRequest(test, "DELETE", "/v3/tracks/"+strconv.Itoa(i), "", 204)
	}
	// Zero the weighting on track 39
	setupRequest(test, "PUT", "/v3/tracks/39/weighting", "0", 200)

	request := basicRequest(test, "GET", "/v3/tracks/random", "")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		test.Error(err)
		return
	}
	var result SearchResultV3
	json.NewDecoder(response.Body).Decode(&result)

	if len(result.Tracks) != 20 {
		test.Errorf("Expected 20 tracks (all copies of track 40), got %d", len(result.Tracks))
	}
	for _, track := range result.Tracks {
		if track.ID != 40 {
			test.Errorf("Expected all random tracks to be id 40, got id %d", track.ID)
		}
	}
}

/**
 * Checks that the q= parameter is rejected with a 400 error
 */
func TestSimpleQueryReturnsError(test *testing.T) {
	makeRequest(test, "GET", "/v3/tracks?q=blue", "", 400, `{"error":"The q parameter is no longer supported. Use p. parameters for predicate-based search.","code":"bad_request"}`, true)
}

/**
 * Checks predicate query (p.key=value) filters to exact tag value match
 */
func TestPredicateQuery(test *testing.T) {
	clearData()

	// Title is Yellow Submarine
	setupRequest(test, "PUT", "/v3/tracks?fingerprint=abc1", `{"url":"http://example.org/track1", "duration": 7,"tags":{"artist":[{"name":"The Beatles"}], "title":[{"name":"Yellow Submarine"}]}}`, 200)
	// Title contains Yellow Submarine (but isn't exactly it)
	setupRequest(test, "PUT", "/v3/tracks?fingerprint=abc2", `{"url":"http://example.org/track2", "duration": 7,"tags":{"artist":[{"name":"The Ladybirds"}], "title":[{"name":"Want to visit a Yellow Submarine"}]}}`, 200)
	// Artist is Yellow Submarine
	setupRequest(test, "PUT", "/v3/tracks?fingerprint=abc3", `{"url":"http://example.org/track3", "duration": 7,"tags":{"artist":[{"name":"Yellow Submarine"}], "title":[{"name":"Love Me Do"}]}}`, 200)
	// No mention of Yellow Submarine
	setupRequest(test, "PUT", "/v3/tracks?fingerprint=abc4", `{"url":"http://example.org/track4", "duration": 7,"tags":{"artist":[{"name":"Robert Johnson"}], "title":[{"name":"Sweet Home Chicago"}]}}`, 200)

	request := basicRequest(test, "GET", "/v3/tracks?p.title=Yellow%20Submarine", "")
	resp, _ := doRawRequest(test, request)
	var result SearchResultV3
	json.NewDecoder(resp.Body).Decode(&result)

	if len(result.Tracks) != 1 {
		test.Errorf("Expected 1 track for p.title=Yellow Submarine, got %d", len(result.Tracks))
	}
	if len(result.Tracks) > 0 && result.Tracks[0].ID != 1 {
		test.Errorf("Expected track 1, got track %d", result.Tracks[0].ID)
	}
}

/**
 * Checks querying for multiple predicates (AND logic)
 */
func TestMultiPredicateQuery(test *testing.T) {
	clearData()

	// Album Now 42, Artist The Corrs
	setupRequest(test, "PUT", "/v3/tracks?fingerprint=abc1", `{"url":"http://example.org/track1", "duration": 7,"tags":{"album":[{"name":"Now 42"}],"artist":[{"name":"The Corrs"}],"title":[{"name":"What Can I Do"}]}}`, 200)
	// Album Now 42, Artist Beautiful South
	setupRequest(test, "PUT", "/v3/tracks?fingerprint=abc2", `{"url":"http://example.org/track2", "duration": 7,"tags":{"album":[{"name":"Now 42"}],"artist":[{"name":"The Beautiful South"}],"title":[{"name":"How Long"}]}}`, 200)
	// Album Now 42, Artist Divine Comedy
	setupRequest(test, "PUT", "/v3/tracks?fingerprint=abc3", `{"url":"http://example.org/track3", "duration": 7,"tags":{"album":[{"name":"Now 42"}],"artist":[{"name":"The Divine Comedy"}],"title":[{"name":"National Express"}]}}`, 200)
	// Album Now 41, Artist Beautiful South
	setupRequest(test, "PUT", "/v3/tracks?fingerprint=abc4", `{"url":"http://example.org/track4", "duration": 7,"tags":{"album":[{"name":"Now 41"}],"artist":[{"name":"The Beautiful South"}],"title":[{"name":"Perfect 10"}]}}`, 200)

	request := basicRequest(test, "GET", "/v3/tracks?p.artist=The%20Beautiful%20South&p.album=Now%2042", "")
	resp, _ := doRawRequest(test, request)
	var result SearchResultV3
	json.NewDecoder(resp.Body).Decode(&result)

	if len(result.Tracks) != 1 {
		test.Errorf("Expected 1 track for Beautiful South + Now 42, got %d", len(result.Tracks))
	}
	if len(result.Tracks) > 0 && result.Tracks[0].ID != 2 {
		test.Errorf("Expected track 2, got track %d", result.Tracks[0].ID)
	}
}

/**
 * Checks querying for tracks missing a predicate (p.key= with empty value)
 */
func TestNullPredicateQuery(test *testing.T) {
	clearData()

	// Track with genre
	setupRequest(test, "PUT", "/v3/tracks?fingerprint=abc1", `{"url":"http://example.org/track1", "duration": 7,"tags":{"artist":[{"name":"The Beatles"}],"title":[{"name":"Yellow Submarine"}],"genre":[{"name":"pop"}]}}`, 200)
	// Track without genre
	setupRequest(test, "PUT", "/v3/tracks?fingerprint=abc2", `{"url":"http://example.org/track2", "duration": 7,"tags":{"artist":[{"name":"The Ladybirds"}],"title":[{"name":"Chirpy Chirpy"}]}}`, 200)
	// Track with genre
	setupRequest(test, "PUT", "/v3/tracks?fingerprint=abc3", `{"url":"http://example.org/track3", "duration": 7,"tags":{"artist":[{"name":"Robert Johnson"}],"title":[{"name":"Sweet Home Chicago"}],"genre":[{"name":"blues"}]}}`, 200)
	// Track without genre
	setupRequest(test, "PUT", "/v3/tracks?fingerprint=abc4", `{"url":"http://example.org/track4", "duration": 7,"tags":{"artist":[{"name":"Vivaldi"}],"title":[{"name":"Spring"}]}}`, 200)

	// Empty value = search for tracks missing this predicate
	request := basicRequest(test, "GET", "/v3/tracks?p.genre=", "")
	resp, _ := doRawRequest(test, request)
	var result SearchResultV3
	json.NewDecoder(resp.Body).Decode(&result)

	if len(result.Tracks) != 2 {
		test.Errorf("Expected 2 tracks without genre, got %d", len(result.Tracks))
	}
	for _, track := range result.Tracks {
		if track.ID == 1 || track.ID == 3 {
			test.Errorf("Track %d has a genre and should not be in null predicate results", track.ID)
		}
	}
}

/**
 * Checks combining null and non-null predicate queries
 */
func TestMixedNullPredicateQuery(test *testing.T) {
	clearData()

	// Track with artist and genre
	setupRequest(test, "PUT", "/v3/tracks?fingerprint=abc1", `{"url":"http://example.org/track1", "duration": 7,"tags":{"artist":[{"name":"The Beatles"}],"title":[{"name":"Yellow Submarine"}],"genre":[{"name":"pop"}]}}`, 200)
	// Track with artist but no genre
	setupRequest(test, "PUT", "/v3/tracks?fingerprint=abc2", `{"url":"http://example.org/track2", "duration": 7,"tags":{"artist":[{"name":"The Beatles"}],"title":[{"name":"Let It Be"}]}}`, 200)
	// Different artist, no genre
	setupRequest(test, "PUT", "/v3/tracks?fingerprint=abc3", `{"url":"http://example.org/track3", "duration": 7,"tags":{"artist":[{"name":"The Rolling Stones"}],"title":[{"name":"Satisfaction"}]}}`, 200)

	// Beatles tracks missing genre
	request := basicRequest(test, "GET", "/v3/tracks?p.artist=The%20Beatles&p.genre=", "")
	resp, _ := doRawRequest(test, request)
	var result SearchResultV3
	json.NewDecoder(resp.Body).Decode(&result)

	if len(result.Tracks) != 1 {
		test.Errorf("Expected 1 Beatles track without genre, got %d", len(result.Tracks))
	}
	if len(result.Tracks) > 0 && result.Tracks[0].ID != 2 {
		test.Errorf("Expected track 2, got track %d", result.Tracks[0].ID)
	}
}

/**
 * Checks pagination when no query is given
 */
func TestAllTracksPagination(test *testing.T) {
	clearData()

	// Create 45 tracks
	for i := 1; i <= 45; i++ {
		id := strconv.Itoa(i)
		escapedURL := url.QueryEscape("http://example.org/track/id" + id)
		setupRequest(test, "PUT", fmt.Sprintf("/v3/tracks?url=%s", escapedURL), `{"fingerprint": "abcde`+id+`", "duration": 350}`, 200)
		setupRequest(test, "PUT", "/v3/tracks/"+id+"/weighting", "4.3", 200)
	}

	pages := map[string]int{"1": 20, "2": 20, "3": 5}
	for page, expectedCount := range pages {
		path := "/v3/tracks/?page=" + page
		request := basicRequest(test, "GET", path, "")
		resp, _ := doRawRequest(test, request)
		var result SearchResultV3
		json.NewDecoder(resp.Body).Decode(&result)

		if len(result.Tracks) != expectedCount {
			test.Errorf("Page %s: expected %d tracks, got %d", page, expectedCount, len(result.Tracks))
		}
		if result.TotalPages != 3 {
			test.Errorf("Page %s: expected 3 total pages, got %d", page, result.TotalPages)
		}
		if result.TotalTracks != 45 {
			test.Errorf("Page %s: expected 45 total tracks, got %d", page, result.TotalTracks)
		}
	}
}

/**
 * Checks pagination works when a query filter is active
 */
func TestTrackQueryPagination(test *testing.T) {
	clearData()

	// Create 45 tracks, all with a searchable title
	for i := 1; i <= 45; i++ {
		id := strconv.Itoa(i)
		escapedURL := url.QueryEscape("http://example.org/track/id" + id)
		setupRequest(test, "PUT", fmt.Sprintf("/v3/tracks?url=%s", escapedURL), `{"fingerprint": "abcde`+id+`", "duration": 350, "tags": {"title": [{"name": "test"}]}}`, 200)
		setupRequest(test, "PUT", "/v3/tracks/"+id+"/weighting", "4.3", 200)
	}

	pages := map[string]int{"1": 20, "2": 20, "3": 5}
	for page, expectedCount := range pages {
		path := "/v3/tracks?p.title=test&page=" + page
		request := basicRequest(test, "GET", path, "")
		resp, _ := doRawRequest(test, request)
		var result SearchResultV3
		json.NewDecoder(resp.Body).Decode(&result)

		if len(result.Tracks) != expectedCount {
			test.Errorf("p.title=test page %s: expected %d tracks, got %d", page, expectedCount, len(result.Tracks))
		}
		if result.TotalPages != 3 {
			test.Errorf("p.title=test page %s: expected 3 total pages, got %d", page, result.TotalPages)
		}
	}
}
