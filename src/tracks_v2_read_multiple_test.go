package main
import (
	"encoding/json"
	"fmt"
	"net/http"
	"io/ioutil"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

/**
 * Checks whether a list of all tracks can be returned
 */
func TestGetAllTracksV2(test *testing.T) {
	clearData()
	track1url := "http://example.org/track/1256"
	escapedTrack1Url := url.QueryEscape(track1url)
	track2url := "http://example.org/track/abcdef"
	escapedTrack2Url := url.QueryEscape(track2url)
	input1Json := `{"fingerprint": "aoecu1234", "duration": 300}`
	input2Json := `{"fingerprint": "blahdebo", "duration": 150}`
	output1Json := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/1256", "trackid": 1, "tags": {},"collections":[], "weighting": 0}`
	output2Json := `{"fingerprint": "blahdebo", "duration": 150, "url": "http://example.org/track/abcdef", "trackid": 2, "tags": {},"collections":[], "weighting": 0}`
	alloutputJson1 := `{"tracks":[{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/1256", "trackid": 1, "tags": {},"collections":[], "weighting": 0}], "totalPages": 1}`
	alloutputJson2 := `{"tracks":[{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/1256", "trackid": 1, "tags": {},"collections":[], "weighting": 0}, {"fingerprint": "blahdebo", "duration": 150, "url": "http://example.org/track/abcdef", "trackid": 2, "tags": {},"collections":[], "weighting": 0}], "totalPages": 1}`
	path1 := fmt.Sprintf("/v2/tracks?url=%s", escapedTrack1Url)
	path2 := fmt.Sprintf("/v2/tracks?url=%s", escapedTrack2Url)
	pathall := "/v2/tracks"
	makeRequest(test, "PUT", path1, input1Json, 200, output1Json, true)
	makeRequest(test, "GET", path1, "", 200, output1Json, true)
	makeRequest(test, "GET", pathall, "", 200, alloutputJson1, true)
	makeRequest(test, "PUT", path2, input2Json, 200, output2Json, true)
	makeRequest(test, "GET", path2, "", 200, output2Json, true)
	makeRequest(test, "GET", pathall, "", 200, alloutputJson2, true)

	makeRequestWithUnallowedMethod(test, pathall, "PUT", []string{"GET"})
}

/**
 * Checks random tracks endpoint
 */
func TestRandomTracksV2(test *testing.T) {
	clearData()
	path := "/v2/tracks/random"
	makeRequest(test, "GET", path, "", 200, `{"tracks":[],"totalPages":0}`, true)

	// Create 40 Tracks
	for i := 1; i <= 40; i++ {
		id := strconv.Itoa(i)
		trackurl := "http://example.org/track/id" + id
		escapedTrackUrl := url.QueryEscape(trackurl)
		trackpath := fmt.Sprintf("/v2/tracks?url=%s", escapedTrackUrl)
		inputJson := `{"fingerprint": "abcde` + id + `", "duration": 350}`
		outputJson := `{"fingerprint": "abcde` + id + `", "duration": 350, "url": "` + trackurl + `", "trackid": ` + id + `, "tags": {},"collections":[], "weighting": 0}`
		makeRequest(test, "PUT", trackpath, inputJson, 200, outputJson, true)
		makeRequest(test, "PUT", "/v2/tracks/"+id+"/weighting", "4.3", 200, "4.3", false)
	}
	url := server.URL + path
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
		test.Errorf("Invalid JSON body: %s for %s", err.Error(), path)
	}
	if len(output.Tracks) != 20 {
		test.Errorf("Wrong number of tracks.  Expected: 20, Actual: %d", len(output.Tracks))
	}
	if output.TotalPages != 1 {
		test.Errorf("Wrong number of totalPages.  Expected: 1, Actual: %d", output.TotalPages)
	}
	if !strings.Contains(response.Header.Get("Cache-Control"), "no-cache") {
		test.Errorf("Random track list is cachable")
	}
	if !strings.Contains(response.Header.Get("Cache-Control"), "max-age=0") {
		test.Errorf("Random track missing max-age of 0")
	}
}

/**
 * Checks how random tracks endpoint handles deletes
 * Creates 40 tracks with weightings, then deletes 38 of them and sets the weighting of track 39 to 0.
 * Random endpoint should only return track 40 every time
 */
func TestRandomTracksDealsWithDeletesV2(test *testing.T) {
	clearData()
	path := "/v2/tracks/random"
	makeRequest(test, "GET", path, "", 200, `{"tracks":[],"totalPages":0}`, true)

	// Create 40 Tracks
	for i := 1; i <= 40; i++ {
		id := strconv.Itoa(i)
		trackurl := "http://example.org/track/id" + id
		escapedTrackUrl := url.QueryEscape(trackurl)
		trackpath := fmt.Sprintf("/v2/tracks?url=%s", escapedTrackUrl)
		inputJson := `{"fingerprint": "abcde` + id + `", "duration": 350}`
		outputJson := `{"fingerprint": "abcde` + id + `", "duration": 350, "url": "` + trackurl + `", "trackid": ` + id + `, "tags": {},"collections":[], "weighting": 0}`
		makeRequest(test, "PUT", trackpath, inputJson, 200, outputJson, true)
		makeRequest(test, "PUT", "/v2/tracks/"+id+"/weighting", "4.3", 200, "4.3", false)
	}
	// Delete 38 of them
	for i := 1; i <= 38; i++ {
		id := strconv.Itoa(i)
		makeRequest(test, "DELETE", "/v2/tracks/" + id, "", 204, "", false)
	}
	// Set the weighting on the 39th to 0
	makeRequest(test, "PUT", "/v2/tracks/39/weighting", "0", 200, "0", false)
	url := server.URL + path
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
		test.Errorf("Invalid JSON body: %s for %s", err.Error(), path)
	}
	if len(output.Tracks) != 20 {
		test.Errorf("Wrong number of tracks.  Expected: 20, Actual: %d", len(output.Tracks))
	}
	for _, track := range output.Tracks {
		assertEqual(test, "Random returned deleted track id", 40, track.ID)
	}
}

/**
 * Checks whether the correct tracks are returned when doing a simple query
 */
func TestSimpleQueryV2(test *testing.T) {
	clearData()

	// Mentions blue in the title
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc1", `{"url":"http://example.org/track1", "duration": 7,"tags":{"artist":"Eiffel 65", "title":"I'm blue"}}`, 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"Eiffel 65", "title":"I'm blue"},"collections":[],"weighting": 0}`, true)
	// Artist is Blue (with a captial B)
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc2", `{"url":"http://example.org/track2", "duration": 7,"tags":{"artist":"Blue", "title":"I can"}}`, 200, `{"fingerprint":"abc2","duration":7,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"Blue", "title":"I can"},"collections":[],"weighting": 0}`, true)
	// No mention of blue
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc3", `{"url":"http://example.org/track3", "duration": 7,"tags":{"artist":"Coldplay", "title":"Yellow"}}`, 200, `{"fingerprint":"abc3","duration":7,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Coldplay", "title":"Yellow"},"collections":[],"weighting": 0}`, true)
	// Genre is blues
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc4", `{"url":"http://example.org/track4", "duration": 7,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"}}`, 200, `{"fingerprint":"abc4","duration":7,"url":"http://example.org/track4","trackid":4,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"},"collections":[],"weighting": 0}`, true)
	// URL contains blue
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc5", `{"url":"http://example.org/blue", "duration": 7}`, 200, `{"fingerprint":"abc5","duration":7,"url":"http://example.org/blue","trackid":5,"tags":{},"collections":[],"weighting": 0}`, true)

	makeRequest(test, "GET", "/v2/tracks?q=blue", "", 200, `{"tracks":[
		{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"Eiffel 65", "title":"I'm blue"},"collections":[],"weighting": 0},
		{"fingerprint":"abc2","duration":7,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"Blue", "title":"I can"},"collections":[],"weighting": 0},
		{"fingerprint":"abc4","duration":7,"url":"http://example.org/track4","trackid":4,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"},"collections":[],"weighting": 0},
		{"fingerprint":"abc5","duration":7,"url":"http://example.org/blue","trackid":5,"tags":{},"collections":[],"weighting": 0}
	], "totalPages": 1}`, true)
}


/**
 * Checks whether the correct tracks are returned when querying for a particular predicate
 */
func TestPredicateQueryV2(test *testing.T) {
	clearData()

	// Title is Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc1", `{"url":"http://example.org/track1", "duration": 7,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"}}`, 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"},"collections":[],"weighting": 0}`, true)
	// Title contains Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc2", `{"url":"http://example.org/track2", "duration": 7,"tags":{"artist":"The Ladybirds", "title":"Want to visit a Yellow Submarine"}}`, 200, `{"fingerprint":"abc2","duration":7,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"The Ladybirds", "title":"Want to visit a Yellow Submarine"},"collections":[],"weighting": 0}`, true)
	// Artist is Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc3", `{"url":"http://example.org/track3", "duration": 7,"tags":{"artist":"Yellow Submarine", "title":"Love Me Do"}}`, 200, `{"fingerprint":"abc3","duration":7,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Yellow Submarine", "title":"Love Me Do"},"collections":[],"weighting": 0}`, true)
	// No mention of Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc4", `{"url":"http://example.org/track4", "duration": 7,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"}}`, 200, `{"fingerprint":"abc4","duration":7,"url":"http://example.org/track4","trackid":4,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"},"collections":[],"weighting": 0}`, true)

	makeRequest(test, "GET", "/v2/tracks?p.title=Yellow%20Submarine", "", 200, `{"tracks":[
		{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"},"collections":[],"weighting": 0}
	], "totalPages": 1}`, true)
}

/**
 * Checks whether the correct tracks are returned when querying for multiple predicates
 */
func TestMultiPredicateQueryV2(test *testing.T) {
	clearData()

	// Album is Now 42
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc1", `{"url":"http://example.org/track1", "duration": 7,"tags":{"album": "Now That's What I Call Music! 42","artist":"The Corrs", "title":"What Can I Do"}}`, 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"album": "Now That's What I Call Music! 42","artist":"The Corrs", "title":"What Can I Do"},"collections":[],"weighting": 0}`, true)
	// Album is Now 42 AND Artist is Beatiful South
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc2", `{"url":"http://example.org/track2", "duration": 7,"tags":{"album": "Now That's What I Call Music! 42", "artist":"The Beautiful South", "title":"How Long's A Tear Take To Dry"}}`, 200, `{"fingerprint":"abc2","duration":7,"url":"http://example.org/track2","trackid":2,"tags":{"album": "Now That's What I Call Music! 42", "artist":"The Beautiful South", "title":"How Long's A Tear Take To Dry"},"collections":[],"weighting": 0}`, true)
	// Album is Now 42
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc3", `{"url":"http://example.org/track3", "duration": 7,"tags":{"album": "Now That's What I Call Music! 42","artist":"The Divine Comedy", "title":"National Express"}}`, 200, `{"fingerprint":"abc3","duration":7,"url":"http://example.org/track3","trackid":3,"tags":{"album": "Now That's What I Call Music! 42","artist":"The Divine Comedy", "title":"National Express"},"collections":[],"weighting": 0}`, true)
	// Artist is Beautiful South
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc4", `{"url":"http://example.org/track4", "duration": 7,"tags":{"album": "Now That's What I Call Music! 41","artist":"The Beautiful South", "title":"Perfect 10"}}`, 200, `{"fingerprint":"abc4","duration":7,"url":"http://example.org/track4","trackid":4,"tags":{"album": "Now That's What I Call Music! 41","artist":"The Beautiful South", "title":"Perfect 10"},"collections":[],"weighting": 0}`, true)

	makeRequest(test, "GET", "/v2/tracks?p.artist=The%20Beautiful%20South&p.album=Now%20That%27s%20What%20I%20Call%20Music!%2042", "", 200, `{"tracks":[
		{"fingerprint":"abc2","duration":7,"url":"http://example.org/track2","trackid":2,"tags":{"album": "Now That's What I Call Music! 42", "artist":"The Beautiful South", "title":"How Long's A Tear Take To Dry"},"collections":[],"weighting": 0}
	], "totalPages": 1}`, true)
}

func TestInvalidQueriesV2(test *testing.T) {
	makeRequestWithUnallowedMethod(test, "/v2/tracks?q=blue", "POST", []string{"GET"})
}


/**
 * Checks the pagination when no query is given
 */
func TestAllTracksPaginationV2(test *testing.T) {
	clearData()

	// Create 45 Tracks
	for i := 1; i <= 45; i++ {
		id := strconv.Itoa(i)
		trackurl := "http://example.org/track/id" + id
		escapedTrackUrl := url.QueryEscape(trackurl)
		trackpath := fmt.Sprintf("/v2/tracks?url=%s", escapedTrackUrl)
		inputJson := `{"fingerprint": "abcde` + id + `", "duration": 350}`
		outputJson := `{"fingerprint": "abcde` + id + `", "duration": 350, "url": "` + trackurl + `", "trackid": ` + id + `, "tags": {},"collections":[], "weighting": 0}`
		makeRequest(test, "PUT", trackpath, inputJson, 200, outputJson, true)
		makeRequest(test, "PUT", "/v2/tracks/"+id+"/weighting", "4.3", 200, "4.3", false)
	}
	pages := map[string]int{
		"1": 20,
		"2": 20,
		"3": 5,
	}
	for page, count := range pages {

		url := server.URL + "/v2/tracks/?page=" + page
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
			test.Errorf("Invalid JSON body: %s for %s", err.Error(), "/v2/tracks/?page="+page)
		}
		if len(output.Tracks) != count {
			test.Errorf("Wrong number of tracks.  Expected: %d, Actual: %d", count, len(output.Tracks))
		}
	}
}

/**
 * Checks pagination when a query is used
 */
func TestTrackQueryPaginationV2(test *testing.T) {
	clearData()

	// Create 45 Tracks
	for i := 1; i <= 45; i++ {
		id := strconv.Itoa(i)
		trackurl := "http://example.org/track/id" + id
		escapedTrackUrl := url.QueryEscape(trackurl)
		trackpath := fmt.Sprintf("/v2/tracks?url=%s", escapedTrackUrl)
		inputJson := `{"fingerprint": "abcde` + id + `", "duration": 350, "tags":{"title":"test"}}`
		outputJson := `{"fingerprint": "abcde` + id + `", "duration": 350, "url": "` + trackurl + `", "trackid": ` + id + `, "tags": {"title":"test"},"collections":[], "weighting": 0}`
		makeRequest(test, "PUT", trackpath, inputJson, 200, outputJson, true)
		makeRequest(test, "PUT", "/v2/tracks/"+id+"/weighting", "4.3", 200, "4.3", false)
	}
	pages := map[string]int{
		"1": 20,
		"2": 20,
		"3": 5,
	}
	for page, count := range pages {

		url := server.URL + "/v2/tracks?q=test&page=" + page
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
			test.Errorf("Invalid JSON body: %s for %s", err.Error(), "/v2/tracks?q=test&page="+page)
		}
		if len(output.Tracks) != count {
			test.Errorf("Wrong number of tracks.  Expected: %d, Actual: %d", count, len(output.Tracks))
		}
		assertEqual(test, "Incorrect Page Count", 3, output.TotalPages)
	}

	for page, count := range pages {

		url := server.URL + "/v2/tracks?p.title=test&page=" + page
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
			test.Errorf("Invalid JSON body: %s for %s", err.Error(), "/v2/tracks?q=test&page="+page)
		}
		if len(output.Tracks) != count {
			test.Errorf("Wrong number of tracks.  Expected: %d, Actual: %d", count, len(output.Tracks))
		}
		assertEqual(test, "Incorrect Page Count", 3, output.TotalPages)
	}
}