package main

import (
	"fmt"
	"net/url"
	"testing"
)

/**
 * Checks whether new tags can be added
 */
func TestAddTag(test *testing.T) {
	clearData()
	path1 := "/tags/1/artist"
	path2 := "/tags/1/album"
	alltagsfortrack := "/tags/1"
	allpredicates := "/predicates"
	predicate1path := "/predicates/artist"
	predicate2path := "/predicates/album"
	makeRequest(test, "GET", path1, "", 404, "Tag Not Found\n", false)
	makeRequest(test, "GET", predicate1path, "", 404, "Predicate Not Found\n", false)
	makeRequest(test, "GET", predicate2path, "", 404, "Predicate Not Found\n", false)
	makeRequest(test, "GET", allpredicates, "", 200, "[]", true)

	trackurl := "http://example.org/track/98765"
	escapedTrackUrl := url.QueryEscape(trackurl)
	trackInput := `{"fingerprint": "aoecu1234", "duration": 300}`
	trackOutput := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/98765", "trackid": 1, "tags": {}, "weighting": 0}`
	trackOutputTagged := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/98765", "trackid": 1, "tags": {"album": "un","artist":"Chumbawamba"}, "weighting": 0}`
	trackpath := fmt.Sprintf("/tracks?url=%s", escapedTrackUrl)
	makeRequest(test, "PUT", trackpath, trackInput, 200, trackOutput, true)
	makeRequest(test, "PUT", predicate1path, `{"id":"artist"}`, 200, `{"id":"artist"}`, true)
	makeRequest(test, "GET", path1, "", 404, "Tag Not Found\n", false)
	makeRequest(test, "GET", allpredicates, "", 200, `[{"id":"artist"}]`, true)

	makeRequest(test, "PUT", path1, "Chumbawamba", 200, "Chumbawamba", false)
	makeRequest(test, "GET", path1, "", 200, "Chumbawamba", false)

	makeRequest(test, "PUT", path2, "wysiwyg", 200, "wysiwyg", false)
	makeRequest(test, "GET", path2, "", 200, "wysiwyg", false)
	makeRequest(test, "GET", predicate2path, "", 200, `{"id":"album"}`, true)
	makeRequest(test, "GET", allpredicates, "", 200, `[{"id":"album"},{"id":"artist"}]`, true)

	makeRequest(test, "PUT", path2, "un", 200, "un", false)
	makeRequest(test, "GET", path2, "", 200, "un", false)
	makeRequest(test, "GET", predicate2path, "", 200, `{"id":"album"}`, true)
	makeRequest(test, "GET", allpredicates, "", 200, `[{"id":"album"},{"id":"artist"}]`, true)
	makeRequest(test, "GET", alltagsfortrack, "", 200, `{"album": "un","artist":"Chumbawamba"}`, true)
	makeRequest(test, "GET", trackpath, "", 200, trackOutputTagged, true)
	makeRequest(test, "GET", "/tracks", "", 200, "["+trackOutputTagged+"]", true)

	makeRequestWithUnallowedMethod(test, alltagsfortrack, "PUT", []string{"GET"})
	makeRequestWithUnallowedMethod(test, path1, "POST", []string{"PUT", "GET", "DELETE"})

	checkRedirect(test, "/tags", "/tracks")
}

/**
 * Checks whether tags can be conditionally added
 */
func TestAddTagIfMissing(test *testing.T) {
	clearData()

	// Create track to add tag to
	trackurl := "http://example.org/track/98765"
	escapedTrackUrl := url.QueryEscape(trackurl)
	trackInput := `{"fingerprint": "aoecu1234", "duration": 300}`
	trackOutput := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/98765", "trackid": 1, "tags": {}, "weighting": 0}`
	trackOutputTagged := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/98765", "trackid": 1, "tags": {"rating":"5"}, "weighting": 0}`
	trackpath := fmt.Sprintf("/tracks?url=%s", escapedTrackUrl)
	makeRequest(test, "PUT", trackpath, trackInput, 200, trackOutput, true)

	// Add Tag (this should work as tag doesn't yet exist)
	path := "/tags/1/rating"
	request := basicRequest(test, "PUT", path, "5")
	request.Header.Add("If-None-Match", "*")
	makeRawRequest(test, request, 200, "5", false)
	makeRequest(test, "GET", path, "", 200, "5", false)

	// Update Tag (this shouldn't have any effect as tag already exists)
	request = basicRequest(test, "PUT", path, "2")
	request.Header.Add("If-None-Match", "*")
	makeRawRequest(test, request, 200, "5", false)
	makeRequest(test, "GET", path, "", 200, "5", false)
	makeRequest(test, "GET", trackpath, "", 200, trackOutputTagged, true)
}

/**
 * Checks whether tags can be deleted
 */
func TestDeleteTag(test *testing.T) {
	clearData()
	artistpath := "/tags/1/artist"
	albumpath := "/tags/1/album"

	trackurl := "http://example.org/track/98765"
	escapedTrackUrl := url.QueryEscape(trackurl)
	trackInput := `{"fingerprint": "aoecu1234", "duration": 300}`
	trackOutput := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/98765", "trackid": 1, "tags": {}, "weighting": 0}`
	trackpath := fmt.Sprintf("/tracks?url=%s", escapedTrackUrl)
	makeRequest(test, "PUT", trackpath, trackInput, 200, trackOutput, true)
	makeRequest(test, "PUT", artistpath, "Chumbawamba", 200, "Chumbawamba", false)
	makeRequest(test, "PUT", albumpath, "wysiwyg", 200, "wysiwyg", false)

	makeRequest(test, "DELETE", albumpath, "whatever", 204, "", false)
	makeRequest(test, "GET", artistpath, "Chumbawamba", 200, "Chumbawamba", false)
	makeRequest(test, "GET", albumpath, "", 404, "Tag Not Found\n", false)

	makeRequest(test, "GET", trackpath, "", 200, `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/98765", "trackid": 1, "tags": {"artist":"Chumbawamba"}, "weighting": 0}`, true)
}

/**
 * Checks whether we error correctly for tags being added to unknown tracks
 */
func TestAddTagForUnknownTrack(test *testing.T) {
	clearData()
	makeRequest(test, "PUT", "/tags/1/artist", "The Corrs", 404, "Unknown Track\n", false)
}