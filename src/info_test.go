package main

import (
	"fmt"
	"net/url"
	"strconv"
	"testing"
	"time"
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
	fiveMinsAgo := time.Now().UTC().Add(time.Minute * -5).Format("2006-01-02T15:04:05.000000")
	fourDaysAgo := time.Now().UTC().Add(time.Hour * 24 * -4).Format("2006-01-02T15:04:05.000000")
	makeRequest(test, "PUT", "/globals/latest_import-timestamp", fiveMinsAgo, 200, fiveMinsAgo, false)
	makeRequest(test, "PUT", "/globals/latest_import-errors", "12", 200, "12", false)
	makeRequest(test, "PUT", "/globals/latest_weightings-timestamp", fourDaysAgo, 200, fourDaysAgo, false)

	expectedOutput := `{
		"system": "lucos_media_metadata_api",
		"checks": {
			"db": {"techDetail":"Does basic SELECT query from database", "ok": true},
			"import": {"techDetail": "Checks whether 'latest_import-timestamp' is within the past 14 days", "ok": true},
			"weightings": {"techDetail": "Checks whether 'latest_weightings-timestamp' is within the past 2 days", "ok": false}
		},
		"metrics": {
			"track-count":{"techDetail":"Number of tracks in database", "value": 37},
			"since-import":{"techDetail":"Seconds since latest completion of import script", "value": 300},
			"import-errors":{"techDetail":"Number of errors from latest completed run of import script", "value": 12},
			"since-weightings":{"techDetail":"Seconds since latest completion of weightings script", "value": 345600}
		},
		"ci":{"circle":"gh/lucas42/lucos_media_metadata_api"}
	}`
	makeRequest(test, "GET", "/_info", "", 200, expectedOutput, true)

	makeRequestWithUnallowedMethod(test, "/_info", "POST", []string{"GET"})
}
