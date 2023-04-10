package main
import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"
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
	makeRequest(test, "PUT", "/v2/tracks/13/weighting", "70", 404, "Track Not Found\n", false)
}



/**
 * Checks whether a track can be updated using its trackid
 */
func TestTrackDeletionV2(test *testing.T) {
	clearData()
	input1Json := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/333", "tags": {"title":"Track One"}}`
	input2Json := `{"fingerprint": "76543", "duration": 150, "url": "http://example.org/track/334", "tags": {"title": "Track Two"}}`
	output1Json := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/333", "trackid": 1, "tags": {"title":"Track One"}, "weighting": 0}`
	output2Json := `{"fingerprint": "76543", "duration": 150, "url": "http://example.org/track/334", "trackid": 2, "tags": {"title": "Track Two"}, "weighting": 0}`
	makeRequest(test, "PUT", "/v2/tracks/1", input1Json, 200, output1Json, true)
	assertEqual(test, "Loganne call", "trackAdded", lastLoganneType)
	makeRequest(test, "PUT", "/v2/tracks/2", input2Json, 200, output2Json, true)
	assertEqual(test, "Loganne call", "trackAdded", lastLoganneType)
	makeRequest(test, "GET", "/v2/tracks/1", "", 200, output1Json, true)
	makeRequest(test, "GET", "/v2/tracks/2", "", 200, output2Json, true)
	makeRequest(test, "DELETE", "/v2/tracks/2", "", 204, "", false)
	assertEqual(test, "Loganne event type", "trackDeleted", lastLoganneType)
	assertEqual(test, "Loganne humanReadable", "Track \"Track Two\" deleted", lastLoganneMessage)
	assertEqual(test, "Loganne track ID", 0, lastLoganneTrack.ID)
	assertEqual(test, "Loganne existingTrack ID", 2, lastLoganneExistingTrack.ID)
	makeRequest(test, "GET", "/v2/tracks/2", "", 404, "Track Not Found\n", false)
	makeRequest(test, "GET", "/v2/tracks/1", "", 200, output1Json, true)
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
	inputAJson := `{"fingerprint": "aoecu1234", "url": "http://example.org/track/444", "duration": 300}`
	outputAJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/444", "trackid": 1, "tags": {}, "weighting": 0}`
	makeRequest(test, "PUT", trackpath, inputAJson, 200, outputAJson, true)
	assertEqual(test, "Loganne call", "trackAdded", lastLoganneType)
	makeRequest(test, "GET", trackpath, "", 200, outputAJson, true)
	inputBJson := `{"tags": {"artist":"Beautiful South", "album": "Carry On Up The Charts"}}`
	outputBJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/444", "trackid": 1, "tags": {"artist":"Beautiful South", "album": "Carry On Up The Charts"}, "weighting": 0}`
	makeRequest(test, "PATCH", trackpath, inputBJson, 200, outputBJson, true)
	assertEqual(test, "Loganne call", "trackUpdated", lastLoganneType)
	makeRequest(test, "GET", trackpath, "", 200, outputBJson, true)
}
/**
 * Checks whether multiple tags can be updated at once when track is identified by URL
 */
func TestUpdateMultipleTagsByURLV2(test *testing.T) {
	clearData()
	trackurl := "http://example.org/track/444"
	escapedTrackUrl := url.QueryEscape(trackurl)
	trackpath := fmt.Sprintf("/v2/tracks?url=%s", escapedTrackUrl)
	inputAJson := `{"fingerprint": "aoecu1234", "url": "http://example.org/track/444", "duration": 300}`
	outputAJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/444", "trackid": 1, "tags": {}, "weighting": 0}`
	makeRequest(test, "PUT", trackpath, inputAJson, 200, outputAJson, true)
	assertEqual(test, "Loganne call", "trackAdded", lastLoganneType)
	makeRequest(test, "GET", trackpath, "", 200, outputAJson, true)
	inputBJson := `{"tags": {"artist":"Beautiful South", "album": "Carry On Up The Charts"}}`
	outputBJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/444", "trackid": 1, "tags": {"artist":"Beautiful South", "album": "Carry On Up The Charts"}, "weighting": 0}`
	makeRequest(test, "PATCH", trackpath, inputBJson, 200, outputBJson, true)
	assertEqual(test, "Loganne call", "trackUpdated", lastLoganneType)
	makeRequest(test, "GET", trackpath, "", 200, outputBJson, true)
}
/**
 * Checks whether a tag can be deleted whilst others are updated
 */
func TestDeleteTagInMultipleV2(test *testing.T) {
	clearData()
	trackpath := "/v2/tracks/1"
	inputAJson := `{"fingerprint": "aoecu1234", "url": "http://example.org/track/444", "duration": 300, "tags": {"artist":"Beautiful South", "album": "Carry On Up The Charts", "genre": "pop"}}`
	outputAJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/444", "trackid": 1, "tags": {"artist":"Beautiful South", "album": "Carry On Up The Charts", "genre": "pop"}, "weighting": 0}`
	makeRequest(test, "PUT", trackpath, inputAJson, 200, outputAJson, true)
	assertEqual(test, "Loganne call", "trackAdded", lastLoganneType)
	makeRequest(test, "GET", trackpath, "", 200, outputAJson, true)
	inputBJson := `{"tags": {"artist":"The Beautiful South", "album": null, "title": "Good As Gold"}}`
	outputBJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/444", "trackid": 1, "tags": {"artist":"The Beautiful South", "title": "Good As Gold", "genre": "pop"}, "weighting": 0}`
	makeRequest(test, "PATCH", trackpath, inputBJson, 200, outputBJson, true)
	assertEqual(test, "Loganne call", "trackUpdated", lastLoganneType)
	makeRequest(test, "GET", trackpath, "", 200, outputBJson, true)
}
/**
 * Checks whether only the missing tags can be added when multiple are specified
 */
func TestUpdateMissingInMultipleTagsV2(test *testing.T) {
	clearData()
	trackpath := "/v2/tracks/1"
	inputAJson := `{"fingerprint": "aoecu1234", "url": "http://example.org/track/444", "duration": 300, "tags": {"title":"Original", "artist": "Has Been Changed"}}`
	outputAJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/444", "trackid": 1, "tags": {"title":"Original", "artist": "Has Been Changed"}, "weighting": 0}`
	makeRequest(test, "PUT", trackpath, inputAJson, 200, outputAJson, true)
	assertEqual(test, "Loganne call", "trackAdded", lastLoganneType)
	makeRequest(test, "GET", trackpath, "", 200, outputAJson, true)
	inputBJson := `{"tags":{"title":"Original", "artist": "Old Artist", "album": "Brand New Album"}}}`
	outputBJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/444", "trackid": 1, "tags": {"title":"Original", "artist": "Has Been Changed", "album": "Brand New Album"}, "weighting": 0}`

	request := basicRequest(test, "PATCH", trackpath, inputBJson)
	request.Header.Add("If-None-Match", "*")
	makeRawRequest(test, request, 200, outputBJson, true)
	assertEqual(test, "Loganne call", "trackUpdated", lastLoganneType)
	makeRequest(test, "GET", trackpath, "", 200, outputBJson, true)
}
/**
 * Checks that if no tags are missing, then loganne doesn't get called
 */
func TestNoMissingTagsDoesntPostToLogannV2(test *testing.T) {
	clearData()
	trackpath := "/v2/tracks/1"
	inputAJson := `{"fingerprint": "aoecu1234", "url": "http://example.org/track/444", "duration": 300, "tags": {"title":"Original", "artist": "Not Changed", "album": "Irrelevant"}}`
	outputJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/444", "trackid": 1, "tags": {"title":"Original", "artist": "Not Changed", "album": "Irrelevant"}, "weighting": 0}`
	makeRequest(test, "PUT", trackpath, inputAJson, 200, outputJson, true)
	assertEqual(test, "Loganne call", "trackAdded", lastLoganneType)
	makeRequest(test, "GET", trackpath, "", 200, outputJson, true)

	inputBJson := `{"fingerprint": "aoecu1234", "url": "http://example.org/track/444", "duration": 300, "tags":{"title":"Original", "artist": "Has Been Changed"}}}`
	lastLoganneType = ""
	request := basicRequest(test, "PUT", trackpath, inputBJson)
	request.Header.Add("If-None-Match", "*")
	makeRawRequest(test, request, 200, outputJson, true)
	assertEqual(test, "Loganne call", "", lastLoganneType) // Shouldn't have logged in this case
	makeRequest(test, "GET", trackpath, "", 200, outputJson, true)
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
 * Checks whether the "added" tag gets set for new tracks when missing from the request
 */
func TestAddedTagV2(test *testing.T) {
	clearData()
	inputJson := `{"url": "http://example.org/track/1256", "duration": 300}`
	path := "/v2/tracks?fingerprint=aoecu1234"
	request := basicRequest(test, "PUT", path, inputJson)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		test.Error(err)
	}

	track := new(Track)
	err = json.NewDecoder(response.Body).Decode(track)
	if err != nil {
		test.Error(err)
	}
	addedTime, err := time.Parse(time.RFC3339, track.Tags["added"])
	if err != nil {
		test.Error(err)
	}
	secondsSinceAdded := time.Since(addedTime).Seconds()
	if secondsSinceAdded > 3 {
		test.Errorf("Added tag is older than expected (%f seconds is more than 3 seconds)", secondsSinceAdded)
	}
}