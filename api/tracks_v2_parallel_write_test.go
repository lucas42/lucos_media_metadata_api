package main
import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"testing"
)

/**
 * Tries creating tracks and updating weightings at the same time
 */
func TestParallelWritesV2(test *testing.T) {
	clearData()
	totalTracks := 1000 // This number is total guess work.  Too low and the DB never locks; too high and the test itself segfaults
	
	// Create lots of tracks (not in parallel, to ensure ids are predictable)
	createTracks(test, totalTracks)

	var wg sync.WaitGroup
	wg.Add(1 + (totalTracks / 20))

	// Do one long running multi-update request
	go updateTracks(test, &wg)

	// Also update all the weightings in batches
	for i := 20; i <= totalTracks; i += 20 {
		go updateWeightingBatch(test, &wg, i-19, i)
	}

	wg.Wait()

	// Once the parallell writes have finished; check all the weightings are as expected
	for i := 1; i <= totalTracks; i++ {
		id := strconv.Itoa(i)
		makeRequest(test, "GET", "/v2/tracks/"+id+"/weighting", "", 200, "73", false)
	}

	expectedInfoOutput := `{
		"system": "lucos_media_metadata_api",
		"checks": {
			"db": {"techDetail":"Does basic SELECT query from database", "ok": true},
			"weighting": {"techDetail":"Does the maximum cumulative weighting value match the sum of all weightings", "ok":true},
			"collections-weighting": {"techDetail":"Whether maximum cumulative weighting for each collection matches the sum of all its weightings", "ok":true}
		},
		"metrics": {
			"track-count": {"techDetail":"Number of tracks in database", "value": `+strconv.Itoa(totalTracks)+`},
			"weighting-drift": {"techDetail":"Difference between maximum cumulativeweighting and the sum of all weightings", "value":0},
			"collections-weighting-drift": {"techDetail":"The number of collections whose maximum cumulative weighting doesn't match the sum of all its weightings", "value":0}
		},
		"ci":{"circle":"gh/lucas42/lucos_media_metadata_api"}
	}`
	makeRequest(test, "GET", "/_info", "", 200, expectedInfoOutput, true)

}

func createTracks(test *testing.T, totalTracks int) {
	
	for i := 1; i <= totalTracks; i++ {
		id := strconv.Itoa(i)
		trackurl := "http://example.org/track/id" + id
		escapedTrackUrl := url.QueryEscape(trackurl)
		trackpath := fmt.Sprintf("/v2/tracks?url=%s", escapedTrackUrl)
		inputJson := `{"fingerprint": "abcde` + id + `", "duration": 350, "tags":{"title":"Yellow Submarine"}}`
		outputJson := `{"fingerprint": "abcde` + id + `", "duration": 350, "url": "` + trackurl + `", "trackid": ` + id + `, "tags": {"title":"Yellow Submarine"},"collections":[], "weighting": 0}`
		makeRequest(test, "PUT", trackpath, inputJson, 200, outputJson, true)
	}
}
func updateTracks(t *testing.T, wg *sync.WaitGroup) {
	defer wg.Done()
	request := basicRequest(t, "PATCH", "/v2/tracks?p.title=Yellow%20Submarine", `{"tags":{"rating":"4"}}`)

	// Due to the asynchronous nature of requests here, it's hard to predict the "correct" response body
	// Most important thing is to check it's not returning a 500 error
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Error(err)
	}
	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		t.Error(err)
	}
	actualResponseBody := string(responseData)

	if response.StatusCode >= 500 {
		t.Errorf("Error code %d returned for updateTracks request, message: %s", response.StatusCode, actualResponseBody)
		return
	}
}

func updateWeightingBatch(test *testing.T, wg *sync.WaitGroup, from int, to int) {
	defer wg.Done()
	for i := from; i <= to; i++ {
		id := strconv.Itoa(i)
		makeRequest(test, "PUT", "/v2/tracks/"+id+"/weighting", "73", 200, "73", false)
	}
}
