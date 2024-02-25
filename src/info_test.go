package main

import (
	"fmt"
	"net/url"
	"strconv"
	"testing"
)

/**
 * Checks whether the /_info endpoint returns expected data
 */
func TestInfoEndpoint(test *testing.T) {
	clearData()

	// Create 37 Tracks
	for i := 1; i <= 37; i++ {
		id := strconv.Itoa(i)
		trackurl := "http://example.org/track/id" + id
		escapedTrackUrl := url.QueryEscape(trackurl)
		trackpath := fmt.Sprintf("/v2/tracks?url=%s", escapedTrackUrl)
		inputJson := `{"fingerprint": "abcde` + id + `", "duration": 350}`
		outputJson := `{"fingerprint": "abcde` + id + `", "duration": 350, "url": "` + trackurl + `", "trackid": ` + id + `, "tags": {},"collections":[], "weighting": 0}`
		makeRequest(test, "PUT", trackpath, inputJson, 200, outputJson, true)
		makeRequest(test, "PUT", "/v2/tracks/"+id+"/weighting", "4.3", 200, "4.3", false)
	}

	expectedOutput := `{
		"system": "lucos_media_metadata_api",
		"checks": {
			"db": {"techDetail":"Does basic SELECT query from database", "ok": true},
			"weighting": {"techDetail":"Does the maximum cumulative weighting value match the sum of all weightings", "ok":true},
			"collections-weighting": {"techDetail":"Whether maximum cumulative weighting for each collection matches the sum of all its weightings", "ok":true}
		},
		"metrics": {
			"track-count": {"techDetail":"Number of tracks in database", "value": 37},
			"weighting-drift": {"techDetail":"Difference between maximum cumulativeweighting and the sum of all weightings", "value":0},
			"collections-weighting-drift": {"techDetail":"The number of collections whose maximum cumulative weighting doesn't match the sum of all its weightings", "value":0}
		},
		"ci":{"circle":"gh/lucas42/lucos_media_metadata_api"}
	}`
	makeRequest(test, "GET", "/_info", "", 200, expectedOutput, true)

	makeRequestWithUnallowedMethod(test, "/_info", "POST", []string{"GET"})

	// Make request with no authentication
	request := basicRequest(test, "GET", "/_info", "")
	request.Header.Del("Authorization")
	makeRawRequest(test, request, 200, expectedOutput, true)
}
