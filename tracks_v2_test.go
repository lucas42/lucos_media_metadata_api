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
 * Checks whether a track can be edited based on its url and retrieved later
 */
func TestCanEditTrackByUrlV2(test *testing.T) {
	clearData()
	trackurl := "http://example.org/track/1256"
	escapedTrackUrl := url.QueryEscape(trackurl)
	inputJson := `{"fingerprint": "aoecu1234", "duration": 300}`
	outputJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/1256", "trackid": 1, "tags": {}, "weighting": 0}`
	path := fmt.Sprintf("/v2/tracks?url=%s", escapedTrackUrl)
	makeRequest(test, "GET", path, "", 404, "Track Not Found\n", false)
	makeRequest(test, "PUT", path, inputJson, 200, outputJson, true)
	assertEqual(test, "Loganne event type", "trackAdded", lastLoganneType)
	assertEqual(test, "Loganne humanReadable", "New Track #1 added", lastLoganneMessage)
	assertEqual(test, "Loganne track ID", 1, lastLoganneTrack.ID)
	assertEqual(test, "Loganne existingTrack ID", 0, lastLoganneExistingTrack.ID)
	makeRequest(test, "GET", path, "", 200, outputJson, true)
	restartServer()
	makeRequest(test, "GET", path, "", 200, outputJson, true)
	makeRequestWithUnallowedMethod(test, path, "POST", []string{"PUT", "GET", "DELETE"})
}

/**
 * Checks whether a track can have its duration updated
 */
func TestCanUpdateDurationV2(test *testing.T) {
	clearData()
	trackurl := "http://example.org/track/333"
	escapedTrackUrl := url.QueryEscape(trackurl)
	inputAJson := `{"fingerprint": "aoecu1234", "duration": 300}`
	inputBJson := `{"fingerprint": "aoecu1234", "duration": 150}`
	outputAJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/333", "trackid": 1, "tags": {}, "weighting": 0}`
	outputBJson := `{"fingerprint": "aoecu1234", "duration": 150, "url": "http://example.org/track/333", "trackid": 1, "tags": {}, "weighting": 0}`
	path := fmt.Sprintf("/v2/tracks?url=%s", escapedTrackUrl)
	makeRequest(test, "GET", path, "", 404, "Track Not Found\n", false)
	makeRequest(test, "PUT", path, inputAJson, 200, outputAJson, true)
	assertEqual(test, "Loganne call", "trackAdded", lastLoganneType)
	makeRequest(test, "PUT", path, inputBJson, 200, outputBJson, true)
	assertEqual(test, "Loganne event type", "trackUpdated", lastLoganneType)
	assertEqual(test, "Loganne humanReadable", "Track #1 updated", lastLoganneMessage)
	assertEqual(test, "Loganne track ID", 1, lastLoganneTrack.ID)
	assertEqual(test, "Loganne existingTrack ID", 1, lastLoganneExistingTrack.ID)
	makeRequest(test, "GET", path, "", 200, outputBJson, true)
}

/**
 * Checks whether a track can be edited based on its fingerprint and retrieved later
 */
func TestCanEditTrackByFingerprintV2(test *testing.T) {
	clearData()
	inputJson := `{"url": "http://example.org/track/1256", "duration": 300}`
	outputJson1 := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/1256", "trackid": 1, "tags": {}, "weighting": 0}`
	outputJson2 := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/cdef", "trackid": 1, "tags": {}, "weighting": 0}`
	path := "/v2/tracks?fingerprint=aoecu1234"
	makeRequest(test, "GET", path, "", 404, "Track Not Found\n", false)
	makeRequest(test, "PUT", path, inputJson, 200, outputJson1, true)
	assertEqual(test, "Loganne call", "trackAdded", lastLoganneType)
	makeRequest(test, "GET", path, "", 200, outputJson1, true)
	restartServer()
	makeRequest(test, "GET", path, "", 200, outputJson1, true)
	makeRequest(test, "PUT", path, `{"url": "http://example.org/track/cdef", "duration": 300}`, 200, outputJson2, true)
	assertEqual(test, "Loganne call", "trackUpdated", lastLoganneType)
	makeRequest(test, "GET", path, "", 200, outputJson2, true)
}

/**
 * Checks whether a track can be partially edited based on its fingerprint
 */
func TestCanPatchTrackByFingerprintV2(test *testing.T) {
	clearData()
	inputJson := `{"url": "http://example.org/track/1256", "duration": 300}`
	outputJson1 := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/1256", "trackid": 1, "tags": {}, "weighting": 0}`
	outputJson2 := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/cdef", "trackid": 1, "tags": {}, "weighting": 0}`
	path := "/v2/tracks?fingerprint=aoecu1234"
	makeRequest(test, "PUT", path, inputJson, 200, outputJson1, true)
	assertEqual(test, "Loganne call", "trackAdded", lastLoganneType)
	makeRequest(test, "GET", path, "", 200, outputJson1, true)
	makeRequest(test, "PATCH", path, `{"url": "http://example.org/track/cdef"}`, 200, outputJson2, true)
	assertEqual(test, "Loganne call", "trackUpdated", lastLoganneType)
	makeRequest(test, "GET", path, "", 200, outputJson2, true)
	restartServer()
	makeRequest(test, "GET", path, "", 200, outputJson2, true)
}
/**
 * Checks whether a track can be partially edited based on its id
 */
func TestCanPatchTrackByIdV2(test *testing.T) {
	clearData()
	inputJson := `{"fingerprint": "aoecu1235", "url": "http://example.org/track/1256", "duration": 300}`
	outputJson1 := `{"fingerprint": "aoecu1235", "duration": 300, "url": "http://example.org/track/1256", "trackid": 1, "tags": {}, "weighting": 0}`
	outputJson2 := `{"fingerprint": "aoecu1235", "duration": 300, "url": "http://example.org/track/cdef", "trackid": 1, "tags": {}, "weighting": 0}`
	pathByUrl := "/v2/tracks?fingerprint=aoecu1235"
	path := "/v2/tracks/1"
	makeRequest(test, "PUT", pathByUrl, inputJson, 200, outputJson1, true)
	assertEqual(test, "Loganne call", "trackAdded", lastLoganneType)
	makeRequest(test, "GET", path, "", 200, outputJson1, true)
	makeRequest(test, "PATCH", path, `{"url": "http://example.org/track/cdef"}`, 200, outputJson2, true)
	assertEqual(test, "Loganne call", "trackUpdated", lastLoganneType)
	makeRequest(test, "GET", path, "", 200, outputJson2, true)
	restartServer()
	makeRequest(test, "GET", path, "", 200, outputJson2, true)
}

/**
 * Checks that non-existant tracks aren't created using PATCH
 */
func TestPatchOnlyUpdatesExistingTracksV2(test *testing.T) {
	path := "/v2/tracks?fingerprint=aoecu1234"
	clearData()
	makeRequest(test, "PATCH", path, `{"url": "http://example.org/track/cdef"}`, 404, "Track Not Found\n", false)
	assertEqual(test, "Loganne shouldn't be called", "", lastLoganneType)
	makeRequest(test, "GET", "/v2/tracks", "", 200, `{"tracks":[],"totalPages":0}`, true)
}

/**
 * Checks that an patch call with an empty object doesn't change anything
 */
func TestEmptyPatchIsInertV2(test *testing.T) {
	clearData()
	trackpath := "/v2/tracks/1"
	inputJson := `{"fingerprint": "aoecu4321", "url": "http://example.org/track/444", "duration": 300}`
	outputJson := `{"fingerprint": "aoecu4321", "duration": 300, "url": "http://example.org/track/444", "trackid": 1, "tags": {}, "weighting": 0}`
	makeRequest(test, "PUT", trackpath, inputJson, 200, outputJson, true)
	assertEqual(test, "Loganne call", "trackAdded", lastLoganneType)
	makeRequest(test, "GET", trackpath, "", 200, outputJson, true)
	emptyInput := `{}`
	lastLoganneType = ""
	makeRequest(test, "PATCH", trackpath, emptyInput, 200, outputJson, true)
	assertEqual(test, "Loganne call", "", lastLoganneType) // Shouldn't have logged in this case
	makeRequest(test, "GET", trackpath, "", 200, outputJson, true)
}

/**
 * Checks whether a track can be updated using its trackid
 */
func TestCanUpdateByIdV2(test *testing.T) {
	clearData()
	track1url := "http://example.org/track/333"
	escapedTrack1Url := url.QueryEscape(track1url)
	track2url := "http://example.org/track/334"
	escapedTrack2Url := url.QueryEscape(track2url)
	inputAJson := `{"fingerprint": "aoecu1234", "duration": 300}`
	inputBJson := `{"fingerprint": "76543", "duration": 150, "url": "http://example.org/track/334"}`
	outputAJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/333", "trackid": 1, "tags": {}, "weighting": 0}`
	outputBJson := `{"fingerprint": "76543", "duration": 150, "url": "http://example.org/track/334", "trackid": 1, "tags": {}, "weighting": 0}`
	path1 := fmt.Sprintf("/v2/tracks?url=%s", escapedTrack1Url)
	path2 := fmt.Sprintf("/v2/tracks?url=%s", escapedTrack2Url)
	path := "/v2/tracks/1"
	makeRequest(test, "GET", path1, "", 404, "Track Not Found\n", false)
	makeRequest(test, "GET", path, "", 404, "Track Not Found\n", false)
	makeRequest(test, "PUT", path1, inputAJson, 200, outputAJson, true)
	assertEqual(test, "Loganne call", "trackAdded", lastLoganneType)
	makeRequest(test, "GET", path1, "", 200, outputAJson, true)
	makeRequest(test, "PUT", path, inputBJson, 200, outputBJson, true)
	assertEqual(test, "Loganne call", "trackUpdated", lastLoganneType)
	makeRequest(test, "GET", path1, "", 404, "Track Not Found\n", false)
	makeRequest(test, "GET", path2, "", 200, outputBJson, true)
	makeRequest(test, "PUT", path, `{"start": "some JSON"`, 400, "unexpected EOF\n", false)
	makeRequest(test, "GET", path, "", 200, outputBJson, true)
}

/**
 * Checks the Track-Action HTTP Header
 */
func TestTrackActionHeaderV2(test *testing.T) {
	clearData()
	path := "/v2/tracks/1"
	inputAJson := `{"fingerprint":"aoecu1234","duration":300,"url":"http://example.org/track/333","trackid":1,"tags":{},"weighting":0}`
	request := basicRequest(test, "PUT", path, inputAJson)
	response := makeRawRequest(test, request, 200, inputAJson, true)
	checkResponseHeader(test, response, "Track-Action", "trackAdded")
	assertEqual(test, "Loganne call", "trackAdded", lastLoganneType)

	inputBJson := `{"fingerprint":"aoecu1234","duration":299,"url":"http://example.org/track/changed","trackid":1,"tags":{},"weighting":0}`
	request = basicRequest(test, "PUT", path, inputBJson)
	response = makeRawRequest(test, request, 200, inputBJson, true)
	checkResponseHeader(test, response, "Track-Action", "trackUpdated")
	assertEqual(test, "Loganne call", "trackUpdated", lastLoganneType)

	lastLoganneType = ""
	request = basicRequest(test, "PUT", path, inputBJson)
	response = makeRawRequest(test, request, 200, inputBJson, true)
	checkResponseHeader(test, response, "Track-Action", "noChange")
	assertEqual(test, "Loganne call", "", lastLoganneType)

	request = basicRequest(test, "DELETE", path, "")
	response = makeRawRequest(test, request, 204, "", false)
	checkResponseHeader(test, response, "Track-Action", "trackDeleted")
	assertEqual(test, "Loganne call", "trackDeleted", lastLoganneType)
}

/**
 * Checks whether a PUT request with missing fields causes an error
 */
func TestCantPutIncompleteDataV2(test *testing.T) {
	clearData()
	inputJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/334"}`
	outputJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/334", "trackid": 1, "tags": {}, "weighting": 0}`
	missingFingerprint := `{"duration": 150, "url": "http://example.org/track/13"}`
	missingUrl := `{"fingerprint": "aoecu9876", "duration": 240}`
	missingDuration := `{"fingerprint": "aoecu2468", "url": "http://example.org/track/87987"}`
	missingFingerprintAndUrl := `{"duration": 17}`
	path := "/v2/tracks/1"
	makeRequest(test, "GET", path, "", 404, "Track Not Found\n", false)
	makeRequest(test, "PUT", path, inputJson, 200, outputJson, true)
	makeRequest(test, "PUT", path, missingFingerprint, 400, "Missing fields \"fingerprint\"\n", false)
	makeRequest(test, "GET", path, "", 200, outputJson, true)
	makeRequest(test, "PUT", path, missingUrl, 400, "Missing fields \"url\"\n", false)
	makeRequest(test, "GET", path, "", 200, outputJson, true)
	makeRequest(test, "PUT", path, missingDuration, 400, "Missing fields \"duration\"\n", false)
	makeRequest(test, "GET", path, "", 200, outputJson, true)
	makeRequest(test, "PUT", path, missingFingerprintAndUrl, 400, "Missing fields \"fingerprint\" and \"url\"\n", false)
	makeRequest(test, "GET", path, "", 200, outputJson, true)
	makeRequest(test, "PUT", path, "{}", 400, "Missing fields \"fingerprint\" and \"url\" and \"duration\"\n", false)
	makeRequest(test, "GET", path, "", 200, outputJson, true)
}

/**
 * Checks whether non-integer track IDs are handled correctly
 */
func TestInvalidTrackIDsV2(test *testing.T) {
	clearData()
	makeRequest(test, "GET", "/v2/tracks/blah", "", 404, "Track Endpoint Not Found\n", false)
	makeRequest(test, "GET", "/v2/tracks/blah/weighting", "", 404, "Track Endpoint Not Found\n", false)
	makeRequest(test, "GET", "/v2/tracks/1/blahing", "", 404, "Track Endpoint Not Found\n", false)
	makeRequest(test, "GET", "/tags/four/artist", "", 400, "Track ID must be an integer\n", false)
}

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
	output1Json := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/1256", "trackid": 1, "tags": {}, "weighting": 0}`
	output2Json := `{"fingerprint": "blahdebo", "duration": 150, "url": "http://example.org/track/abcdef", "trackid": 2, "tags": {}, "weighting": 0}`
	alloutputJson1 := `{"tracks":[{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/1256", "trackid": 1, "tags": {}, "weighting": 0}], "totalPages": 1}`
	alloutputJson2 := `{"tracks":[{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/1256", "trackid": 1, "tags": {}, "weighting": 0}, {"fingerprint": "blahdebo", "duration": 150, "url": "http://example.org/track/abcdef", "trackid": 2, "tags": {}, "weighting": 0}], "totalPages": 1}`
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
 * Checks whether a track can be updated using its trackid
 */
func TestTrackDeletionV2(test *testing.T) {
	clearData()
	input1Json := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/333"}`
	input2Json := `{"fingerprint": "76543", "duration": 150, "url": "http://example.org/track/334"}`
	output1Json := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/333", "trackid": 1, "tags": {}, "weighting": 0}`
	output2Json := `{"fingerprint": "76543", "duration": 150, "url": "http://example.org/track/334", "trackid": 2, "tags": {}, "weighting": 0}`
	output1TagsJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/333", "trackid": 1, "tags": {"title":"Track One"}, "weighting": 0}`
	output2TagsJson := `{"fingerprint": "76543", "duration": 150, "url": "http://example.org/track/334", "trackid": 2, "tags": {"title": "Track Two"}, "weighting": 0}`
	makeRequest(test, "PUT", "/v2/tracks/1", input1Json, 200, output1Json, true)
	assertEqual(test, "Loganne call", "trackAdded", lastLoganneType)
	makeRequest(test, "PUT", "/tags/1/title", "Track One", 200, "Track One", false)
	makeRequest(test, "GET", "/v2/tracks/1", "", 200, output1TagsJson, true)
	makeRequest(test, "PUT", "/v2/tracks/2", input2Json, 200, output2Json, true)
	makeRequest(test, "PUT", "/tags/2/title", "Track Two", 200, "Track Two", false)
	makeRequest(test, "GET", "/v2/tracks/2", "", 200, output2TagsJson, true)
	makeRequest(test, "DELETE", "/v2/tracks/2", "", 204, "", false)
	assertEqual(test, "Loganne event type", "trackDeleted", lastLoganneType)
	assertEqual(test, "Loganne humanReadable", "Track \"Track Two\" deleted", lastLoganneMessage)
	assertEqual(test, "Loganne track ID", 0, lastLoganneTrack.ID)
	assertEqual(test, "Loganne existingTrack ID", 2, lastLoganneExistingTrack.ID)
	makeRequest(test, "GET", "/v2/tracks/2", "", 404, "Track Not Found\n", false)
	makeRequest(test, "GET", "/tags/2", "", 200, "{}", true)
	makeRequest(test, "GET", "/v2/tracks/1", "", 200, output1TagsJson, true)
}

func TestCantDeleteAllV2(test *testing.T) {
	clearData()
	inputAJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/333"}`
	inputBJson := `{"fingerprint": "76543", "duration": 150, "url": "http://example.org/track/334"}`
	outputAJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/333", "trackid": 1, "tags": {}, "weighting": 0}`
	outputBJson := `{"fingerprint": "76543", "duration": 150, "url": "http://example.org/track/334", "trackid": 2, "tags": {}, "weighting": 0}`
	makeRequest(test, "PUT", "/v2/tracks/1", inputAJson, 200, outputAJson, true)
	makeRequest(test, "PUT", "/v2/tracks/2", inputBJson, 200, outputBJson, true)
	makeRequestWithUnallowedMethod(test, "/v2/tracks", "DELETE", []string{"GET"})
	makeRequest(test, "GET", "/v2/tracks/2", "", 200, outputBJson, true)
	makeRequest(test, "GET", "/v2/tracks/1", "", 200, outputAJson, true)
}



/**
 * Checks whether multiple tags can be updated at once when track is identified by track ID
 */
func TestUpdateMultipleTagsV2(test *testing.T) {
	clearData()
	trackpath := "/v2/tracks/1"
	artistpath := "/tags/1/artist"
	albumpath := "/tags/1/album"
	inputAJson := `{"fingerprint": "aoecu1234", "url": "http://example.org/track/444", "duration": 300}`
	outputAJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/444", "trackid": 1, "tags": {}, "weighting": 0}`
	makeRequest(test, "PUT", trackpath, inputAJson, 200, outputAJson, true)
	assertEqual(test, "Loganne call", "trackAdded", lastLoganneType)
	makeRequest(test, "GET", trackpath, "", 200, outputAJson, true)
	makeRequest(test, "GET", artistpath, "", 404, "Tag Not Found\n", false)
	makeRequest(test, "GET", albumpath, "", 404, "Tag Not Found\n", false)
	inputBJson := `{"tags": {"artist":"Beautiful South", "album": "Carry On Up The Charts"}}`
	outputBJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/444", "trackid": 1, "tags": {"artist":"Beautiful South", "album": "Carry On Up The Charts"}, "weighting": 0}`
	makeRequest(test, "PATCH", trackpath, inputBJson, 200, outputBJson, true)
	assertEqual(test, "Loganne call", "trackUpdated", lastLoganneType)
	makeRequest(test, "GET", artistpath, "", 200, "Beautiful South", false)
	makeRequest(test, "GET", albumpath, "", 200, "Carry On Up The Charts", false)
}
/**
 * Checks whether multiple tags can be updated at once when track is identified by URL
 */
func TestUpdateMultipleTagsByURLV2(test *testing.T) {
	clearData()
	trackurl := "http://example.org/track/444"
	escapedTrackUrl := url.QueryEscape(trackurl)
	trackpath := fmt.Sprintf("/v2/tracks?url=%s", escapedTrackUrl)
	artistpath := "/tags/1/artist"
	albumpath := "/tags/1/album"
	inputAJson := `{"fingerprint": "aoecu1234", "url": "http://example.org/track/444", "duration": 300}`
	outputAJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/444", "trackid": 1, "tags": {}, "weighting": 0}`
	makeRequest(test, "PUT", trackpath, inputAJson, 200, outputAJson, true)
	assertEqual(test, "Loganne call", "trackAdded", lastLoganneType)
	makeRequest(test, "GET", trackpath, "", 200, outputAJson, true)
	makeRequest(test, "GET", artistpath, "", 404, "Tag Not Found\n", false)
	makeRequest(test, "GET", albumpath, "", 404, "Tag Not Found\n", false)
	inputBJson := `{"tags": {"artist":"Beautiful South", "album": "Carry On Up The Charts"}}`
	outputBJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/444", "trackid": 1, "tags": {"artist":"Beautiful South", "album": "Carry On Up The Charts"}, "weighting": 0}`
	makeRequest(test, "PATCH", trackpath, inputBJson, 200, outputBJson, true)
	assertEqual(test, "Loganne call", "trackUpdated", lastLoganneType)
	makeRequest(test, "GET", artistpath, "", 200, "Beautiful South", false)
	makeRequest(test, "GET", albumpath, "", 200, "Carry On Up The Charts", false)
}
/**
 * Checks whether a tag can be deleted whilst others are updated
 */
func TestDeleteTagInMultipleV2(test *testing.T) {
	clearData()
	trackpath := "/v2/tracks/1"
	titlepath := "/tags/1/title"
	artistpath := "/tags/1/artist"
	albumpath := "/tags/1/album"
	genrepath := "/tags/1/genre"
	inputAJson := `{"fingerprint": "aoecu1234", "url": "http://example.org/track/444", "duration": 300, "tags": {"artist":"Beautiful South", "album": "Carry On Up The Charts", "genre": "pop"}}`
	outputAJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/444", "trackid": 1, "tags": {"artist":"Beautiful South", "album": "Carry On Up The Charts", "genre": "pop"}, "weighting": 0}`
	makeRequest(test, "PUT", trackpath, inputAJson, 200, outputAJson, true)
	assertEqual(test, "Loganne call", "trackAdded", lastLoganneType)
	makeRequest(test, "GET", trackpath, "", 200, outputAJson, true)
	makeRequest(test, "GET", titlepath, "", 404, "Tag Not Found\n", false)
	makeRequest(test, "GET", artistpath, "", 200, "Beautiful South", false)
	makeRequest(test, "GET", albumpath, "", 200, "Carry On Up The Charts", false)
	makeRequest(test, "GET", genrepath, "", 200, "pop", false)
	inputBJson := `{"tags": {"artist":"The Beautiful South", "album": null, "title": "Good As Gold"}}`
	outputBJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/444", "trackid": 1, "tags": {"artist":"The Beautiful South", "title": "Good As Gold", "genre": "pop"}, "weighting": 0}`
	makeRequest(test, "PATCH", trackpath, inputBJson, 200, outputBJson, true)
	assertEqual(test, "Loganne call", "trackUpdated", lastLoganneType)
	makeRequest(test, "GET", titlepath, "", 200, "Good As Gold", false)
	makeRequest(test, "GET", artistpath, "", 200, "The Beautiful South", false)
	makeRequest(test, "GET", albumpath, "", 404, "Tag Not Found\n", false)
	makeRequest(test, "GET", genrepath, "", 200, "pop", false)
}
/**
 * Checks whether only the missing tags can be added when multiple are specified
 */
func TestUpdateMissingInMultipleTagsV2(test *testing.T) {
	clearData()
	trackpath := "/v2/tracks/1"
	titlepath := "/tags/1/title"
	artistpath := "/tags/1/artist"
	albumpath := "/tags/1/album"
	inputAJson := `{"fingerprint": "aoecu1234", "url": "http://example.org/track/444", "duration": 300, "tags": {"title":"Original", "artist": "Has Been Changed"}}`
	outputAJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/444", "trackid": 1, "tags": {"title":"Original", "artist": "Has Been Changed"}, "weighting": 0}`
	makeRequest(test, "PUT", trackpath, inputAJson, 200, outputAJson, true)
	assertEqual(test, "Loganne call", "trackAdded", lastLoganneType)
	makeRequest(test, "GET", trackpath, "", 200, outputAJson, true)
	makeRequest(test, "GET", titlepath, "", 200, "Original", false)
	makeRequest(test, "GET", artistpath, "", 200, "Has Been Changed", false)
	makeRequest(test, "GET", albumpath, "", 404, "Tag Not Found\n", false)
	inputBJson := `{"tags":{"title":"Original", "artist": "Old Artist", "album": "Brand New Album"}}}`
	outputBJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/444", "trackid": 1, "tags": {"title":"Original", "artist": "Has Been Changed", "album": "Brand New Album"}, "weighting": 0}`

	request := basicRequest(test, "PATCH", trackpath, inputBJson)
	request.Header.Add("If-None-Match", "*")
	makeRawRequest(test, request, 200, outputBJson, true)
	assertEqual(test, "Loganne call", "trackUpdated", lastLoganneType)
	makeRequest(test, "GET", titlepath, "", 200, "Original", false)
	makeRequest(test, "GET", artistpath, "", 200, "Has Been Changed", false)
	makeRequest(test, "GET", albumpath, "", 200, "Brand New Album", false)
}
/**
 * Checks that if no tags are missing, then loganne doesn't get called
 */
func TestNoMissingTagsDoesntPostToLogannV2(test *testing.T) {
	clearData()
	trackpath := "/v2/tracks/1"
	titlepath := "/tags/1/title"
	artistpath := "/tags/1/artist"
	albumpath := "/tags/1/album"
	inputAJson := `{"fingerprint": "aoecu1234", "url": "http://example.org/track/444", "duration": 300, "tags": {"title":"Original", "artist": "Not Changed", "album": "Irrelevant"}}`
	outputJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/444", "trackid": 1, "tags": {"title":"Original", "artist": "Not Changed", "album": "Irrelevant"}, "weighting": 0}`
	makeRequest(test, "PUT", trackpath, inputAJson, 200, outputJson, true)
	assertEqual(test, "Loganne call", "trackAdded", lastLoganneType)
	makeRequest(test, "GET", trackpath, "", 200, outputJson, true)
	makeRequest(test, "GET", titlepath, "", 200, "Original", false)
	makeRequest(test, "GET", artistpath, "", 200, "Not Changed", false)
	makeRequest(test, "GET", albumpath, "", 200, "Irrelevant", false)

	inputBJson := `{"fingerprint": "aoecu1234", "url": "http://example.org/track/444", "duration": 300, "tags":{"title":"Original", "artist": "Has Been Changed"}}}`
	lastLoganneType = ""
	request := basicRequest(test, "PUT", trackpath, inputBJson)
	request.Header.Add("If-None-Match", "*")
	makeRawRequest(test, request, 200, outputJson, true)
	assertEqual(test, "Loganne call", "", lastLoganneType) // Shouldn't have logged in this case
	makeRequest(test, "GET", trackpath, "", 200, outputJson, true)
	makeRequest(test, "GET", titlepath, "", 200, "Original", false)
	makeRequest(test, "GET", artistpath, "", 200, "Not Changed", false)
	makeRequest(test, "GET", albumpath, "", 200, "Irrelevant", false)
}

/**
 * Checks whether a track can have its weighting updated
 */
func TestCanUpdateWeightingV2(test *testing.T) {
	clearData()
	path := "/v2/tracks/1/weighting"
	makeRequest(test, "GET", path, "", 404, "Track Not Found\n", false)

	// Create track
	trackurl := "http://example.org/track/eui4536"
	escapedTrackUrl := url.QueryEscape(trackurl)
	trackpath := fmt.Sprintf("/v2/tracks?url=%s", escapedTrackUrl)
	outputJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/eui4536", "trackid": 1, "tags": {}, "weighting": 0}`
	makeRequest(test, "PUT", trackpath, `{"fingerprint": "aoecu1234", "duration": 300}`, 200, outputJson, true)

	makeRequest(test, "GET", path, "", 200, "0", false)
	makeRequest(test, "PUT", path, "bob", 400, "Weighting must be a number\n", false)
	makeRequest(test, "PUT", path, "3", 200, "3", false)
	makeRequest(test, "GET", path, "", 200, "3", false)
	restartServer()
	makeRequest(test, "GET", path, "", 200, "3", false)
	makeRequest(test, "PUT", path, "5.22", 200, "5.22", false)
	makeRequest(test, "GET", path, "", 200, "5.22", false)


	trackOutputJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/eui4536", "trackid": 1, "tags": {}, "weighting": 5.22}`
	makeRequest(test, "GET", trackpath, "", 200, trackOutputJson, true)

	pathall := "/v2/tracks"
	alloutputJson := `{"tracks":[{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/eui4536", "trackid": 1, "tags": {}, "weighting": 5.22}],"totalPages":1}`
	makeRequest(test, "GET", pathall, "", 200, alloutputJson, true)

	makeRequestWithUnallowedMethod(test, path, "POST", []string{"PUT", "GET"})

	// Test that zeroes are handled as a valid weighting, and not just null
	makeRequest(test, "PUT", path, "0", 200, "0", false)
	zeroTrackOutputJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/eui4536", "trackid": 1, "tags": {}, "weighting": 0}`
	makeRequest(test, "GET", trackpath, "", 200, zeroTrackOutputJson, true)
}

/**
 * Checks whether tracks with duplicate trackurls are handled nicely
 */
func TestDuplicateURLV2(test *testing.T) {
	clearData()
	oldTrackUrl := "http://example.org/oldurl"
	newTrackUrl := "http://example.org/newurl"
	track1Path := "/v2/tracks?fingerprint=track1"
	track2Path := "/v2/tracks?fingerprint=track2"
	track3Path := "/v2/tracks?fingerprint=track3"

	// Set up 2 tracks with different URLs
	makeRequest(test, "PUT", track1Path, `{"url":"` + oldTrackUrl + `", "duration": 7}`, 200, `{"fingerprint":"track1","duration":7,"url":"` + oldTrackUrl + `","trackid":1,"tags":{},"weighting": 0}`, true)
	makeRequest(test, "PUT", track2Path, `{"url":"` + newTrackUrl + `", "duration": 17}`, 200, `{"fingerprint":"track2","duration":17,"url":"` + newTrackUrl + `","trackid":2,"tags":{},"weighting": 0}`, true)

	// Try to update first track to have same URL as second using PATCH
	makeRequest(test, "PATCH", track1Path, `{"url":"` + newTrackUrl + `"}`, 400, "Duplicate: track 2 has same url\n", false)

	// Try to update first track to have same URL as second using PUT
	makeRequest(test, "PUT", track1Path, `{"url":"` + newTrackUrl + `", "duration": 7}`, 400, "Duplicate: track 2 has same url\n", false)

	// Try to create a new track with the same URL as the either of the others
	makeRequest(test, "PUT", track3Path, `{"url":"` + oldTrackUrl + `", "duration": 27}`, 400, "Duplicate: track 1 has same url\n", false)
	makeRequest(test, "PUT", track3Path, `{"url":"` + newTrackUrl + `", "duration": 27}`, 400, "Duplicate: track 2 has same url\n", false)

}

/**
 * Checks whether tracks with duplicate fingerprints are handled nicely
 */
func TestDuplicateFingerprintV2(test *testing.T) {
	clearData()
	fingerprint1 := "ascorc562hsrch"
	fingerprint2 := "lhlacehu43rcphalc"

	track1Path := "/v2/tracks?url="+url.QueryEscape("http://example.org/track1")
	track2Path := "/v2/tracks?url="+url.QueryEscape("http://example.org/track2")
	track3Path := "/v2/tracks?url="+url.QueryEscape("http://example.org/track3")

	// Set up 2 tracks with different fingerprints
	makeRequest(test, "PUT", track1Path, `{"fingerprint":"` + fingerprint1 + `", "duration": 7}`, 200, `{"fingerprint":"` + fingerprint1 + `","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{},"weighting": 0}`, true)
	makeRequest(test, "PUT", track2Path, `{"fingerprint":"` + fingerprint2 + `", "duration": 17}`, 200, `{"fingerprint":"` + fingerprint2 + `","duration":17,"url":"http://example.org/track2","trackid":2,"tags":{},"weighting": 0}`, true)

	// Try to update first track to have same fingerprint as second using PUT
	makeRequest(test, "PUT", track2Path, `{"fingerprint":"` + fingerprint1 + `", "duration": 17}`, 400, "Duplicate: track 1 has same fingerprint\n", false)

	// Try to update first track to have same fingerprint as second using PUT
	makeRequest(test, "PATCH", track2Path, `{"fingerprint":"` + fingerprint1 + `"}`, 400, "Duplicate: track 1 has same fingerprint\n", false)

	// Try to create a new track with the same fingerprint as the either of the others
	makeRequest(test, "PUT", track3Path, `{"fingerprint":"` + fingerprint2 + `", "duration": 17}`, 400, "Duplicate: track 2 has same fingerprint\n", false)
	makeRequest(test, "PUT", track3Path, `{"fingerprint":"` + fingerprint1 + `", "duration": 17}`, 400, "Duplicate: track 1 has same fingerprint\n", false)

}

/**
 * Checks random tracks endpoint
 */
func TestRandomTracksV2(test *testing.T) {
	clearData()
	path := "/v2/tracks/random"
	makeRequest(test, "GET", path, "", 200, "[]", true)

	// Create 40 Tracks
	for i := 1; i <= 40; i++ {
		id := strconv.Itoa(i)
		trackurl := "http://example.org/track/id" + id
		escapedTrackUrl := url.QueryEscape(trackurl)
		trackpath := fmt.Sprintf("/v2/tracks?url=%s", escapedTrackUrl)
		inputJson := `{"fingerprint": "abcde` + id + `", "duration": 350}`
		outputJson := `{"fingerprint": "abcde` + id + `", "duration": 350, "url": "` + trackurl + `", "trackid": ` + id + `, "tags": {}, "weighting": 0}`
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

	var output []interface{}
	err = json.Unmarshal(responseData, &output)
	if err != nil {
		test.Errorf("Invalid JSON body: %s for %s", err.Error(), path)
	}
	if len(output) != 20 {
		test.Errorf("Wrong number of tracks.  Expected: 20, Actual: %d", len(output))
	}
	if !strings.Contains(response.Header.Get("Cache-Control"), "no-cache") {
		test.Errorf("Random track list is cachable")
	}
	if !strings.Contains(response.Header.Get("Cache-Control"), "max-age=0") {
		test.Errorf("Random track missing max-age of 0")
	}
}


/**
 * Checks whether the correct tracks are returned when doing a simple query
 */
func TestSimpleQueryV2(test *testing.T) {
	clearData()

	// Mentions blue in the title
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc1", `{"url":"http://example.org/track1", "duration": 7,"tags":{"artist":"Eiffel 65", "title":"I'm blue"}}`, 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"Eiffel 65", "title":"I'm blue"},"weighting": 0}`, true)
	// Artist is Blue (with a captial B)
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc2", `{"url":"http://example.org/track2", "duration": 7,"tags":{"artist":"Blue", "title":"I can"}}`, 200, `{"fingerprint":"abc2","duration":7,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"Blue", "title":"I can"},"weighting": 0}`, true)
	// No mention of blue
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc3", `{"url":"http://example.org/track3", "duration": 7,"tags":{"artist":"Coldplay", "title":"Yellow"}}`, 200, `{"fingerprint":"abc3","duration":7,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Coldplay", "title":"Yellow"},"weighting": 0}`, true)
	// Genre is blues
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc4", `{"url":"http://example.org/track4", "duration": 7,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"}}`, 200, `{"fingerprint":"abc4","duration":7,"url":"http://example.org/track4","trackid":4,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"},"weighting": 0}`, true)
	// URL contains blue
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc5", `{"url":"http://example.org/blue", "duration": 7}`, 200, `{"fingerprint":"abc5","duration":7,"url":"http://example.org/blue","trackid":5,"tags":{},"weighting": 0}`, true)

	makeRequest(test, "GET", "/v2/tracks?q=blue", "", 200, `{"tracks":[
		{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"Eiffel 65", "title":"I'm blue"},"weighting": 0},
		{"fingerprint":"abc2","duration":7,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"Blue", "title":"I can"},"weighting": 0},
		{"fingerprint":"abc4","duration":7,"url":"http://example.org/track4","trackid":4,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"},"weighting": 0},
		{"fingerprint":"abc5","duration":7,"url":"http://example.org/blue","trackid":5,"tags":{},"weighting": 0}
	], "totalPages": 1}`, true)
}

/**
 * NOCOMMIT!!
 * 
 * Need to consider whether a response of type {tracks:[], totalPages: n} is good enough for the same root path which normally returns an array of tracks
 **/
 func TestStopV2(test *testing.T) {
 	test.Error("NOCOMMIT, see source comment")
 }

/**
 * Checks whether the correct tracks are returned when querying for a particular predicate
 */
func TestPredicateQueryV2(test *testing.T) {
	clearData()

	// Title is Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc1", `{"url":"http://example.org/track1", "duration": 7,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"}}`, 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"},"weighting": 0}`, true)
	// Title contains Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc2", `{"url":"http://example.org/track2", "duration": 7,"tags":{"artist":"The Ladybirds", "title":"Want to visit a Yellow Submarine"}}`, 200, `{"fingerprint":"abc2","duration":7,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"The Ladybirds", "title":"Want to visit a Yellow Submarine"},"weighting": 0}`, true)
	// Artist is Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc3", `{"url":"http://example.org/track3", "duration": 7,"tags":{"artist":"Yellow Submarine", "title":"Love Me Do"}}`, 200, `{"fingerprint":"abc3","duration":7,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Yellow Submarine", "title":"Love Me Do"},"weighting": 0}`, true)
	// No mention of Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc4", `{"url":"http://example.org/track4", "duration": 7,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"}}`, 200, `{"fingerprint":"abc4","duration":7,"url":"http://example.org/track4","trackid":4,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"},"weighting": 0}`, true)

	makeRequest(test, "GET", "/v2/tracks?p.title=Yellow%20Submarine", "", 200, `{"tracks":[
		{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"},"weighting": 0}
	], "totalPages": 1}`, true)
}

/**
 * Checks whether the correct tracks are returned when querying for multiple predicates
 */
func TestMultiPredicateQueryV2(test *testing.T) {
	clearData()

	// Album is Now 42
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc1", `{"url":"http://example.org/track1", "duration": 7,"tags":{"album": "Now That's What I Call Music! 42","artist":"The Corrs", "title":"What Can I Do"}}`, 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"album": "Now That's What I Call Music! 42","artist":"The Corrs", "title":"What Can I Do"},"weighting": 0}`, true)
	// Album is Now 42 AND Artist is Beatiful South
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc2", `{"url":"http://example.org/track2", "duration": 7,"tags":{"album": "Now That's What I Call Music! 42", "artist":"The Beautiful South", "title":"How Long's A Tear Take To Dry"}}`, 200, `{"fingerprint":"abc2","duration":7,"url":"http://example.org/track2","trackid":2,"tags":{"album": "Now That's What I Call Music! 42", "artist":"The Beautiful South", "title":"How Long's A Tear Take To Dry"},"weighting": 0}`, true)
	// Album is Now 42
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc3", `{"url":"http://example.org/track3", "duration": 7,"tags":{"album": "Now That's What I Call Music! 42","artist":"The Divine Comedy", "title":"National Express"}}`, 200, `{"fingerprint":"abc3","duration":7,"url":"http://example.org/track3","trackid":3,"tags":{"album": "Now That's What I Call Music! 42","artist":"The Divine Comedy", "title":"National Express"},"weighting": 0}`, true)
	// Artist is Beautiful South
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc4", `{"url":"http://example.org/track4", "duration": 7,"tags":{"album": "Now That's What I Call Music! 41","artist":"The Beautiful South", "title":"Perfect 10"}}`, 200, `{"fingerprint":"abc4","duration":7,"url":"http://example.org/track4","trackid":4,"tags":{"album": "Now That's What I Call Music! 41","artist":"The Beautiful South", "title":"Perfect 10"},"weighting": 0}`, true)

	makeRequest(test, "GET", "/v2/tracks?p.artist=The%20Beautiful%20South&p.album=Now%20That%27s%20What%20I%20Call%20Music!%2042", "", 200, `{"tracks":[
		{"fingerprint":"abc2","duration":7,"url":"http://example.org/track2","trackid":2,"tags":{"album": "Now That's What I Call Music! 42", "artist":"The Beautiful South", "title":"How Long's A Tear Take To Dry"},"weighting": 0}
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
		outputJson := `{"fingerprint": "abcde` + id + `", "duration": 350, "url": "` + trackurl + `", "trackid": ` + id + `, "tags": {}, "weighting": 0}`
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
		outputJson := `{"fingerprint": "abcde` + id + `", "duration": 350, "url": "` + trackurl + `", "trackid": ` + id + `, "tags": {"title":"test"}, "weighting": 0}`
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