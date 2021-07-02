package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"testing"
)

/**
 * Checks whether the correct tracks are returned from search
 */
func TestSimpleSearch(test *testing.T) {
	clearData()

	// Mentions blue in the title
	makeRequest(test, "PUT", "/tracks?fingerprint=abc1", `{"url":"http://example.org/track1", "duration": 7,"tags":{"artist":"Eiffel 65", "title":"I'm blue"}}`, 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"Eiffel 65", "title":"I'm blue"},"weighting": 0}`, true)
	// Artist is Blue (with a captial B)
	makeRequest(test, "PUT", "/tracks?fingerprint=abc2", `{"url":"http://example.org/track2", "duration": 7,"tags":{"artist":"Blue", "title":"I can"}}`, 200, `{"fingerprint":"abc2","duration":7,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"Blue", "title":"I can"},"weighting": 0}`, true)
	// No mention of blue
	makeRequest(test, "PUT", "/tracks?fingerprint=abc3", `{"url":"http://example.org/track3", "duration": 7,"tags":{"artist":"Coldplay", "title":"Yellow"}}`, 200, `{"fingerprint":"abc3","duration":7,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Coldplay", "title":"Yellow"},"weighting": 0}`, true)
	// Genre is blues
	makeRequest(test, "PUT", "/tracks?fingerprint=abc4", `{"url":"http://example.org/track4", "duration": 7,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"}}`, 200, `{"fingerprint":"abc4","duration":7,"url":"http://example.org/track4","trackid":4,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"},"weighting": 0}`, true)
	// URL contains blue
	makeRequest(test, "PUT", "/tracks?fingerprint=abc5", `{"url":"http://example.org/blue", "duration": 7}`, 200, `{"fingerprint":"abc5","duration":7,"url":"http://example.org/blue","trackid":5,"tags":{},"weighting": 0}`, true)

	makeRequest(test, "GET", "/search?q=blue", "", 200, `{"tracks":[
		{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"Eiffel 65", "title":"I'm blue"},"weighting": 0},
		{"fingerprint":"abc2","duration":7,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"Blue", "title":"I can"},"weighting": 0},
		{"fingerprint":"abc4","duration":7,"url":"http://example.org/track4","trackid":4,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"},"weighting": 0},
		{"fingerprint":"abc5","duration":7,"url":"http://example.org/blue","trackid":5,"tags":{},"weighting": 0}
	]}`, true)
}

/**
 * Checks whether the correct tracks are returned when searching for a particular predicate
 */
func TestPredicateSearch(test *testing.T) {
	clearData()

	// Title is Yellow Submarine
	makeRequest(test, "PUT", "/tracks?fingerprint=abc1", `{"url":"http://example.org/track1", "duration": 7,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"}}`, 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"},"weighting": 0}`, true)
	// Title contains Yellow Submarine
	makeRequest(test, "PUT", "/tracks?fingerprint=abc2", `{"url":"http://example.org/track2", "duration": 7,"tags":{"artist":"The Ladybirds", "title":"Want to visit a Yellow Submarine"}}`, 200, `{"fingerprint":"abc2","duration":7,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"The Ladybirds", "title":"Want to visit a Yellow Submarine"},"weighting": 0}`, true)
	// Artist is Yellow Submarine
	makeRequest(test, "PUT", "/tracks?fingerprint=abc3", `{"url":"http://example.org/track3", "duration": 7,"tags":{"artist":"Yellow Submarine", "title":"Love Me Do"}}`, 200, `{"fingerprint":"abc3","duration":7,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Yellow Submarine", "title":"Love Me Do"},"weighting": 0}`, true)
	// No mention of Yellow Submarine
	makeRequest(test, "PUT", "/tracks?fingerprint=abc4", `{"url":"http://example.org/track4", "duration": 7,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"}}`, 200, `{"fingerprint":"abc4","duration":7,"url":"http://example.org/track4","trackid":4,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"},"weighting": 0}`, true)

	makeRequest(test, "GET", "/search?p.title=Yellow%20Submarine", "", 200, `{"tracks":[
		{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"},"weighting": 0}
	]}`, true)
}

/**
 * Checks whether the correct tracks are returned when searching for multiple predicates
 */
func TestMultiPredicateSearch(test *testing.T) {
	clearData()

	// Album is Now 42
	makeRequest(test, "PUT", "/tracks?fingerprint=abc1", `{"url":"http://example.org/track1", "duration": 7,"tags":{"album": "Now That's What I Call Music! 42","artist":"The Corrs", "title":"What Can I Do"}}`, 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"album": "Now That's What I Call Music! 42","artist":"The Corrs", "title":"What Can I Do"},"weighting": 0}`, true)
	// Album is Now 42 AND Artist is Beatiful South
	makeRequest(test, "PUT", "/tracks?fingerprint=abc2", `{"url":"http://example.org/track2", "duration": 7,"tags":{"album": "Now That's What I Call Music! 42", "artist":"The Beautiful South", "title":"How Long's A Tear Take To Dry"}}`, 200, `{"fingerprint":"abc2","duration":7,"url":"http://example.org/track2","trackid":2,"tags":{"album": "Now That's What I Call Music! 42", "artist":"The Beautiful South", "title":"How Long's A Tear Take To Dry"},"weighting": 0}`, true)
	// Album is Now 42
	makeRequest(test, "PUT", "/tracks?fingerprint=abc3", `{"url":"http://example.org/track3", "duration": 7,"tags":{"album": "Now That's What I Call Music! 42","artist":"The Divine Comedy", "title":"National Express"}}`, 200, `{"fingerprint":"abc3","duration":7,"url":"http://example.org/track3","trackid":3,"tags":{"album": "Now That's What I Call Music! 42","artist":"The Divine Comedy", "title":"National Express"},"weighting": 0}`, true)
	// Artist is Beautiful South
	makeRequest(test, "PUT", "/tracks?fingerprint=abc4", `{"url":"http://example.org/track4", "duration": 7,"tags":{"album": "Now That's What I Call Music! 41","artist":"The Beautiful South", "title":"Perfect 10"}}`, 200, `{"fingerprint":"abc4","duration":7,"url":"http://example.org/track4","trackid":4,"tags":{"album": "Now That's What I Call Music! 41","artist":"The Beautiful South", "title":"Perfect 10"},"weighting": 0}`, true)

	makeRequest(test, "GET", "/search?p.artist=The%20Beautiful%20South&p.album=Now%20That%27s%20What%20I%20Call%20Music!%2042", "", 200, `{"tracks":[
		{"fingerprint":"abc2","duration":7,"url":"http://example.org/track2","trackid":2,"tags":{"album": "Now That's What I Call Music! 42", "artist":"The Beautiful South", "title":"How Long's A Tear Take To Dry"},"weighting": 0}
	]}`, true)
}

func TestInvalidRequestsToSearch(test *testing.T) {
	makeRequestWithUnallowedMethod(test, "/search?q=blue", "POST", []string{"GET"})
	makeRequest(test, "GET", "/search", "", 400, "No query given\n", false)
	makeRequest(test, "GET", "/search?q=", "", 400, "No query given\n", false)
}

/**
 * Checks the pagination of the search endpoint
 */
func TestSearchPagination(test *testing.T) {
	clearData()

	// Create 45 Tracks
	for i := 1; i <= 45; i++ {
		id := strconv.Itoa(i)
		trackurl := "http://example.org/track/id" + id
		escapedTrackUrl := url.QueryEscape(trackurl)
		trackpath := fmt.Sprintf("/tracks?url=%s", escapedTrackUrl)
		inputJson := `{"fingerprint": "abcde` + id + `", "duration": 350, "tags":{"title":"test"}}`
		outputJson := `{"fingerprint": "abcde` + id + `", "duration": 350, "url": "` + trackurl + `", "trackid": ` + id + `, "tags": {"title":"test"}, "weighting": 0}`
		makeRequest(test, "PUT", trackpath, inputJson, 200, outputJson, true)
		makeRequest(test, "PUT", "/tracks/"+id+"/weighting", "4.3", 200, "4.3", false)
	}
	pages := map[string]int{
		"1": 20,
		"2": 20,
		"3": 5,
	}
	for page, count := range pages {

		url := server.URL + "/search?q=test&page=" + page
		response, err := http.Get(url)
		if err != nil {
			test.Error(err)
		}
		responseData, err := ioutil.ReadAll(response.Body)
		if err != nil {
			test.Error(err)
		}

		expectedResponseCode := 200
		if response.StatusCode != expectedResponseCode {
			test.Errorf("Got response code %d, expected %d for %s", response.StatusCode, expectedResponseCode, url)
		}

		var output SearchResult
		err = json.Unmarshal(responseData, &output)
		if err != nil {
			test.Errorf("Invalid JSON body: %s for %s", err.Error(), "/search?q=test&page="+page)
		}
		if len(output.Tracks) != count {
			test.Errorf("Wrong number of tracks.  Expected: %d, Actual: %d", count, len(output.Tracks))
		}
	}
}