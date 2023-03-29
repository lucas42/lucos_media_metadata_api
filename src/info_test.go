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
		trackpath := fmt.Sprintf("/tracks?url=%s", escapedTrackUrl)
		inputJson := `{"fingerprint": "abcde` + id + `", "duration": 350}`
		outputJson := `{"fingerprint": "abcde` + id + `", "duration": 350, "url": "` + trackurl + `", "trackid": ` + id + `, "tags": {}, "weighting": 0}`
		makeRequest(test, "PUT", trackpath, inputJson, 200, outputJson, true)
		makeRequest(test, "PUT", "/tracks/"+id+"/weighting", "4.3", 200, "4.3", false)
	}

	expectedOutput := `{
		"system": "lucos_media_metadata_api",
		"checks": {
			"db": {"techDetail":"Does basic SELECT query from database", "ok": true}
		},
		"metrics": {
			"track-count":{"techDetail":"Number of tracks in database", "value": 37}
		},
		"ci":{"circle":"gh/lucas42/lucos_media_metadata_api"}
	}`
	makeRequest(test, "GET", "/_info", "", 200, expectedOutput, true)

	makeRequestWithUnallowedMethod(test, "/_info", "POST", []string{"GET"})
}
