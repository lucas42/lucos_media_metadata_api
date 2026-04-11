package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"testing"
)

/**
 * Tries creating tracks and updating weightings at the same time.
 * This stress-tests SQLite's WAL mode and concurrent-write handling.
 */
func TestParallelWrites(test *testing.T) {
	clearData()
	totalTracks := 1000 // Too low and the DB never locks; too high and the test may segfault

	// Create tracks sequentially so IDs are predictable
	createTracksV3(test, totalTracks)

	var wg sync.WaitGroup
	wg.Add(1 + (totalTracks / 20))

	// One long-running bulk-update request
	go updateTracksV3(test, &wg)

	// Update all weightings in batches in parallel
	for i := 20; i <= totalTracks; i += 20 {
		go updateWeightingBatchV3(test, &wg, i-19, i)
	}

	wg.Wait()

	// Verify all weightings are set correctly
	for i := 1; i <= totalTracks; i++ {
		id := strconv.Itoa(i)
		makeRequest(test, "GET", "/v3/tracks/"+id+"/weighting", "", 200, "73", false)
	}

	expectedInfoOutput := `{
		"system": "lucos_media_metadata_api",
		"title": "Media Metadata API",
		"checks": {
			"db": {"techDetail":"Does basic SELECT query from database", "ok": true},
			"weighting": {"techDetail":"Does the maximum cumulative weighting value match the sum of all weightings", "ok":true},
			"collections-weighting": {"techDetail":"Whether maximum cumulative weighting for each collection matches the sum of all its weightings", "ok":true},
			"uri-integrity": {"techDetail":"Tags with URI-dependent predicates all have a URI set", "ok":true}
		},
		"metrics": {
			"track-count": {"techDetail":"Number of tracks in database", "value": ` + strconv.Itoa(totalTracks) + `},
			"weighting-drift": {"techDetail":"Difference between maximum cumulativeweighting and the sum of all weightings", "value":0},
			"collections-weighting-drift": {"techDetail":"The number of collections whose maximum cumulative weighting doesn't match the sum of all its weightings", "value":0},
			"tags-missing-uris": {"techDetail":"Number of tags with a URI-dependent predicate but no URI", "value":0}
		},
		"ci":{"circle":"gh/lucas42/lucos_media_metadata_api"}
	}`
	makeRequest(test, "GET", "/_info", "", 200, expectedInfoOutput, true)
}

func createTracksV3(test *testing.T, totalTracks int) {
	for i := 1; i <= totalTracks; i++ {
		id := strconv.Itoa(i)
		trackurl := "http://example.org/track/id" + id
		escapedTrackUrl := url.QueryEscape(trackurl)
		trackpath := fmt.Sprintf("/v3/tracks?url=%s", escapedTrackUrl)
		inputJson := `{"fingerprint": "abcde` + id + `", "duration": 350, "tags":{"title":[{"name":"Yellow Submarine"}]}}`
		setupRequest(test, "PUT", trackpath, inputJson, 200)
	}
}

func updateTracksV3(t *testing.T, wg *sync.WaitGroup) {
	defer wg.Done()
	request := basicRequest(t, "PATCH", "/v3/tracks?p.title=Yellow%20Submarine", `{"tags":{"rating":[{"name":"4"}]}}`)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Error(err)
		return
	}
	if response.StatusCode >= 500 {
		t.Errorf("Error code %d returned for updateTracksV3 request", response.StatusCode)
	}
}

func updateWeightingBatchV3(test *testing.T, wg *sync.WaitGroup, from int, to int) {
	defer wg.Done()
	for i := from; i <= to; i++ {
		id := strconv.Itoa(i)
		makeRequest(test, "PUT", "/v3/tracks/"+id+"/weighting", "73", 200, "73", false)
	}
}
