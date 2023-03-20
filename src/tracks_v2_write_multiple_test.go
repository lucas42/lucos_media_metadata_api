package main
import (
	"fmt"
	"net/url"
	"strconv"
	"testing"
)

/**
 * Checks whether tracks can be updated in bulk based on a predicate
 */
 func TestPatchTracksByPredicateV2(test *testing.T) {
	clearData()

	// Title is Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc1", `{"url":"http://example.org/track1", "duration": 7,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"}}`, 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"},"weighting": 0}`, true)
	// Title contains Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc2", `{"url":"http://example.org/track2", "duration": 7,"tags":{"artist":"The Ladybirds", "title":"Want to visit a Yellow Submarine"}}`, 200, `{"fingerprint":"abc2","duration":7,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"The Ladybirds", "title":"Want to visit a Yellow Submarine"},"weighting": 0}`, true)
	// Artist is Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc3", `{"url":"http://example.org/track3", "duration": 7,"tags":{"artist":"Yellow Submarine", "title":"Love Me Do"}}`, 200, `{"fingerprint":"abc3","duration":7,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Yellow Submarine", "title":"Love Me Do"},"weighting": 0}`, true)
	// No mention of Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc4", `{"url":"http://example.org/track4", "duration": 7,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"}}`, 200, `{"fingerprint":"abc4","duration":7,"url":"http://example.org/track4","trackid":4,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"},"weighting": 0}`, true)
	// Title is also Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc5", `{"url":"http://example.org/track5", "duration": 7,"tags":{"artist":"Panpipes Cover Band", "title":"Yellow Submarine", "genre": "panpipes"}}`, 200, `{"fingerprint":"abc5","duration":7,"url":"http://example.org/track5","trackid":5,"tags":{"artist":"Panpipes Cover Band", "title":"Yellow Submarine", "genre": "panpipes"},"weighting": 0}`, true)

	restartServer() // Clear Loganne counters etc
	request := basicRequest(test, "PATCH", "/v2/tracks?p.title=Yellow%20Submarine", `{"tags": {"genre":"Maritime Songs"}}`)
	response := makeRawRequest(test, request, 200, `{"tracks":[{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles","genre":"Maritime Songs","title":"Yellow Submarine"},"weighting":0},{"fingerprint":"abc5","duration":7,"url":"http://example.org/track5","trackid":5,"tags":{"artist":"Panpipes Cover Band","genre":"Maritime Songs","title":"Yellow Submarine"},"weighting":0}],"totalPages":1}`, true)
	checkResponseHeader(test, response, "Track-Action", "tracksUpdated")
	assertEqual(test, "Loganne call", "tracksUpdated", lastLoganneType)
	assertEqual(test, "Number of Loganne requests", 3, loganneRequestCount) // One request for each track changed, plus one for the bulk change

	// Ensure the tracks which match have been updated
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc1", "", 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine","genre":"Maritime Songs"},"weighting": 0}`, true)
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc5", "", 200, `{"fingerprint":"abc5","duration":7,"url":"http://example.org/track5","trackid":5,"tags":{"artist":"Panpipes Cover Band", "title":"Yellow Submarine","genre":"Maritime Songs"},"weighting": 0}`, true)

	// Ensure the tracks which don't match haven't changed
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc2", "", 200, `{"fingerprint":"abc2","duration":7,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"The Ladybirds", "title":"Want to visit a Yellow Submarine"},"weighting": 0}`, true)
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc3", "", 200, `{"fingerprint":"abc3","duration":7,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Yellow Submarine", "title":"Love Me Do"},"weighting": 0}`, true)
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc4", "", 200, `{"fingerprint":"abc4","duration":7,"url":"http://example.org/track4","trackid":4,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"},"weighting": 0}`, true)

}

/**
 * Checks whether tracks can be updated in bulk based on a predicate, limited to those with missing data
 */
 func TestPatchTracksMissingByPredicateV2(test *testing.T) {
	clearData()

	// Title is Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc1", `{"url":"http://example.org/track1", "duration": 7,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"}}`, 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"},"weighting": 0}`, true)
	// Title contains Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc2", `{"url":"http://example.org/track2", "duration": 7,"tags":{"artist":"The Ladybirds", "title":"Want to visit a Yellow Submarine"}}`, 200, `{"fingerprint":"abc2","duration":7,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"The Ladybirds", "title":"Want to visit a Yellow Submarine"},"weighting": 0}`, true)
	// Artist is Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc3", `{"url":"http://example.org/track3", "duration": 7,"tags":{"artist":"Yellow Submarine", "title":"Love Me Do"}}`, 200, `{"fingerprint":"abc3","duration":7,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Yellow Submarine", "title":"Love Me Do"},"weighting": 0}`, true)
	// No mention of Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc4", `{"url":"http://example.org/track4", "duration": 7,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"}}`, 200, `{"fingerprint":"abc4","duration":7,"url":"http://example.org/track4","trackid":4,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"},"weighting": 0}`, true)
	// Title is also Yellow Submarine, but already has a genre set
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc5", `{"url":"http://example.org/track5", "duration": 7,"tags":{"artist":"Panpipes Cover Band", "title":"Yellow Submarine", "genre": "panpipes"}}`, 200, `{"fingerprint":"abc5","duration":7,"url":"http://example.org/track5","trackid":5,"tags":{"artist":"Panpipes Cover Band", "title":"Yellow Submarine", "genre": "panpipes"},"weighting": 0}`, true)

	restartServer() // Clear Loganne counters etc
	request := basicRequest(test, "PATCH", "/v2/tracks?p.title=Yellow%20Submarine", `{"tags": {"genre":"Maritime Songs"}}`)
	request.Header.Add("If-None-Match", "*")
	response := makeRawRequest(test, request, 200, `{"tracks":[{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles","genre":"Maritime Songs","title":"Yellow Submarine"},"weighting":0}],"totalPages":1}`, true)
	checkResponseHeader(test, response, "Track-Action", "tracksUpdated")
	assertEqual(test, "Loganne call", "tracksUpdated", lastLoganneType)
	assertEqual(test, "Number of Loganne requests", 2, loganneRequestCount) // One request for each track changed, plus one for the bulk change

	// Ensure the tracks which match have been updated
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc1", "", 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine","genre":"Maritime Songs"},"weighting": 0}`, true)

	// Ensure the tracks which don't match haven't changed
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc2", "", 200, `{"fingerprint":"abc2","duration":7,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"The Ladybirds", "title":"Want to visit a Yellow Submarine"},"weighting": 0}`, true)
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc3", "", 200, `{"fingerprint":"abc3","duration":7,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Yellow Submarine", "title":"Love Me Do"},"weighting": 0}`, true)
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc4", "", 200, `{"fingerprint":"abc4","duration":7,"url":"http://example.org/track4","trackid":4,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"},"weighting": 0}`, true)
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc5", "", 200, `{"fingerprint":"abc5","duration":7,"url":"http://example.org/track5","trackid":5,"tags":{"artist":"Panpipes Cover Band", "title":"Yellow Submarine","genre":"panpipes"},"weighting": 0}`, true)

}

/**
 * Checks whether tracks can be updated in bulk based multiple predicates
 */
 func TestPatchTracksByMultiplePredicatesV2(test *testing.T) {
	clearData()

	// Title is Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc1", `{"url":"http://example.org/track1", "duration": 7,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"}}`, 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"},"weighting": 0}`, true)
	// Title contains Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc2", `{"url":"http://example.org/track2", "duration": 7,"tags":{"artist":"The Ladybirds", "title":"Want to visit a Yellow Submarine"}}`, 200, `{"fingerprint":"abc2","duration":7,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"The Ladybirds", "title":"Want to visit a Yellow Submarine"},"weighting": 0}`, true)
	// Artist is Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc3", `{"url":"http://example.org/track3", "duration": 7,"tags":{"artist":"Yellow Submarine", "title":"Love Me Do"}}`, 200, `{"fingerprint":"abc3","duration":7,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Yellow Submarine", "title":"Love Me Do"},"weighting": 0}`, true)
	// No mention of Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc4", `{"url":"http://example.org/track4", "duration": 7,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"}}`, 200, `{"fingerprint":"abc4","duration":7,"url":"http://example.org/track4","trackid":4,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"},"weighting": 0}`, true)
	// Title is also Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc5", `{"url":"http://example.org/track5", "duration": 7,"tags":{"artist":"Panpipes Cover Band", "title":"Yellow Submarine", "genre": "panpipes"}}`, 200, `{"fingerprint":"abc5","duration":7,"url":"http://example.org/track5","trackid":5,"tags":{"artist":"Panpipes Cover Band", "title":"Yellow Submarine", "genre": "panpipes"},"weighting": 0}`, true)

	restartServer() // Clear Loganne counters etc
	request := basicRequest(test, "PATCH", "/v2/tracks?p.title=Yellow%20Submarine&p.artist=Panpipes%20Cover%20Band", `{"tags": {"genre":"Maritime Songs"}}`)
	response := makeRawRequest(test, request, 200, `{"tracks":[{"fingerprint":"abc5","duration":7,"url":"http://example.org/track5","trackid":5,"tags":{"artist":"Panpipes Cover Band","genre":"Maritime Songs","title":"Yellow Submarine"},"weighting":0}],"totalPages":1}`, true)
	checkResponseHeader(test, response, "Track-Action", "tracksUpdated")
	assertEqual(test, "Loganne call", "tracksUpdated", lastLoganneType)
	assertEqual(test, "Number of Loganne requests", 2, loganneRequestCount) // One request for each track changed, plus one for the bulk change

	// Ensure the tracks which match have been updated
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc5", "", 200, `{"fingerprint":"abc5","duration":7,"url":"http://example.org/track5","trackid":5,"tags":{"artist":"Panpipes Cover Band", "title":"Yellow Submarine","genre":"Maritime Songs"},"weighting": 0}`, true)

	// Ensure the tracks which don't match haven't changed
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc1", "", 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"},"weighting": 0}`, true)
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc2", "", 200, `{"fingerprint":"abc2","duration":7,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"The Ladybirds", "title":"Want to visit a Yellow Submarine"},"weighting": 0}`, true)
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc3", "", 200, `{"fingerprint":"abc3","duration":7,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Yellow Submarine", "title":"Love Me Do"},"weighting": 0}`, true)
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc4", "", 200, `{"fingerprint":"abc4","duration":7,"url":"http://example.org/track4","trackid":4,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"},"weighting": 0}`, true)

}

/**
 * Checks whether tracks can be updated in bulk based on a search query
 */
 func TestPatchTracksByQuery(test *testing.T) {
	clearData()

	// Title is Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc1", `{"url":"http://example.org/track1", "duration": 7,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"}}`, 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"},"weighting": 0}`, true)
	// Title contains Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc2", `{"url":"http://example.org/track2", "duration": 7,"tags":{"artist":"The Ladybirds", "title":"Want to visit a Yellow Submarine"}}`, 200, `{"fingerprint":"abc2","duration":7,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"The Ladybirds", "title":"Want to visit a Yellow Submarine"},"weighting": 0}`, true)
	// Artist is Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc3", `{"url":"http://example.org/track3", "duration": 7,"tags":{"artist":"Yellow Submarine", "title":"Love Me Do"}}`, 200, `{"fingerprint":"abc3","duration":7,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Yellow Submarine", "title":"Love Me Do"},"weighting": 0}`, true)
	// No mention of Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc4", `{"url":"http://example.org/track4", "duration": 7,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"}}`, 200, `{"fingerprint":"abc4","duration":7,"url":"http://example.org/track4","trackid":4,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"},"weighting": 0}`, true)
	// Title is also Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc5", `{"url":"http://example.org/track5", "duration": 7,"tags":{"artist":"Panpipes Cover Band", "title":"Yellow Submarine", "genre": "panpipes"}}`, 200, `{"fingerprint":"abc5","duration":7,"url":"http://example.org/track5","trackid":5,"tags":{"artist":"Panpipes Cover Band", "title":"Yellow Submarine", "genre": "panpipes"},"weighting": 0}`, true)

	restartServer() // Clear Loganne counters etc
	request := basicRequest(test, "PATCH", "/v2/tracks?q=Yellow%20Submarine", `{"tags": {"genre":"Maritime Songs"}}`)
	response := makeRawRequest(test, request, 200, `{"tracks":[{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles","genre":"Maritime Songs","title":"Yellow Submarine"},"weighting":0},{"fingerprint":"abc2","duration":7,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"The Ladybirds","genre":"Maritime Songs","title":"Want to visit a Yellow Submarine"},"weighting":0},{"fingerprint":"abc3","duration":7,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Yellow Submarine","genre":"Maritime Songs","title":"Love Me Do"},"weighting":0},{"fingerprint":"abc5","duration":7,"url":"http://example.org/track5","trackid":5,"tags":{"artist":"Panpipes Cover Band","genre":"Maritime Songs","title":"Yellow Submarine"},"weighting":0}],"totalPages":1}`, true)
	checkResponseHeader(test, response, "Track-Action", "tracksUpdated")
	assertEqual(test, "Loganne call", "tracksUpdated", lastLoganneType)
	assertEqual(test, "Number of Loganne requests", 5, loganneRequestCount) // One request for each track changed, plus one for the bulk change

	// Ensure the tracks which match have been updated
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc1", "", 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine","genre":"Maritime Songs"},"weighting": 0}`, true)
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc2", "", 200, `{"fingerprint":"abc2","duration":7,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"The Ladybirds", "title":"Want to visit a Yellow Submarine","genre":"Maritime Songs"},"weighting": 0}`, true)
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc3", "", 200, `{"fingerprint":"abc3","duration":7,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Yellow Submarine", "title":"Love Me Do","genre":"Maritime Songs"},"weighting": 0}`, true)
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc5", "", 200, `{"fingerprint":"abc5","duration":7,"url":"http://example.org/track5","trackid":5,"tags":{"artist":"Panpipes Cover Band", "title":"Yellow Submarine","genre":"Maritime Songs"},"weighting": 0}`, true)

	// Ensure the tracks which don't match haven't changed
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc4", "", 200, `{"fingerprint":"abc4","duration":7,"url":"http://example.org/track4","trackid":4,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"},"weighting": 0}`, true)

}

/**
 * Checks that trying to do a PUT request on a bulk query returns with an Unallowed Method error response
 */
func TestCantPutInBulk(test *testing.T) {
	makeRequestWithUnallowedMethod(test, "/v2/tracks?p.title=Yellow%20Submarine", "PUT", []string{"PATCH", "GET"})
}

/**
 * Checks that a unique field can't be changed as part of a bulk update, to avoid clashes
 */
func TestCantBulkUpdateIdentifiers(test *testing.T) {
	makeRequest(test, "PATCH", "/v2/tracks?q=Yellow", `{"url": "http://example.org/track7"}`, 400, "Can't bulk update url\n", false)
	makeRequest(test, "PATCH", "/v2/tracks?q=Blue", `{"trackid": 7}`, 400, "Can't bulk update id\n", false)
	makeRequest(test, "PATCH", "/v2/tracks?q=Green", `{"fingerprint": "abc7"}`, 400, "Can't bulk update fingerprint\n", false)
}
/**
 * Checks bulk duration change works as expected.  (Though not really sure when this'd be useful)
 */
func TestBulkUpdateDuration(test *testing.T) {
	clearData()

	// Title is Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc1", `{"url":"http://example.org/track1", "duration": 7,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"}}`, 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"},"weighting": 0}`, true)
	// Title contains Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc2", `{"url":"http://example.org/track2", "duration": 7,"tags":{"artist":"The Ladybirds", "title":"Want to visit a Yellow Submarine"}}`, 200, `{"fingerprint":"abc2","duration":7,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"The Ladybirds", "title":"Want to visit a Yellow Submarine"},"weighting": 0}`, true)
	// Artist is Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc3", `{"url":"http://example.org/track3", "duration": 7,"tags":{"artist":"Yellow Submarine", "title":"Love Me Do"}}`, 200, `{"fingerprint":"abc3","duration":7,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Yellow Submarine", "title":"Love Me Do"},"weighting": 0}`, true)
	// No mention of Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc4", `{"url":"http://example.org/track4", "duration": 7,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"}}`, 200, `{"fingerprint":"abc4","duration":7,"url":"http://example.org/track4","trackid":4,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"},"weighting": 0}`, true)
	// Title is also Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc5", `{"url":"http://example.org/track5", "duration": 7,"tags":{"artist":"Panpipes Cover Band", "title":"Yellow Submarine", "genre": "panpipes"}}`, 200, `{"fingerprint":"abc5","duration":7,"url":"http://example.org/track5","trackid":5,"tags":{"artist":"Panpipes Cover Band", "title":"Yellow Submarine", "genre": "panpipes"},"weighting": 0}`, true)

	restartServer() // Clear Loganne counters etc
	request := basicRequest(test, "PATCH", "/v2/tracks?q=Yellow%20Submarine", `{"duration": 137}`)
	response := makeRawRequest(test, request, 200, `{"tracks":[{"fingerprint":"abc1","duration":137,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles","title":"Yellow Submarine"},"weighting":0},{"fingerprint":"abc2","duration":137,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"The Ladybirds","title":"Want to visit a Yellow Submarine"},"weighting":0},{"fingerprint":"abc3","duration":137,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Yellow Submarine","title":"Love Me Do"},"weighting":0},{"fingerprint":"abc5","duration":137,"url":"http://example.org/track5","trackid":5,"tags":{"artist":"Panpipes Cover Band","genre":"panpipes","title":"Yellow Submarine"},"weighting":0}],"totalPages":1}`, true)
	checkResponseHeader(test, response, "Track-Action", "tracksUpdated")
	assertEqual(test, "Loganne call", "tracksUpdated", lastLoganneType)
	assertEqual(test, "Number of Loganne requests", 5, loganneRequestCount) // One request for each track changed, plus one for the bulk change

	// Ensure the tracks which match have been updated
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc1", "", 200, `{"fingerprint":"abc1","duration":137,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"},"weighting": 0}`, true)
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc2", "", 200, `{"fingerprint":"abc2","duration":137,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"The Ladybirds", "title":"Want to visit a Yellow Submarine"},"weighting": 0}`, true)
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc3", "", 200, `{"fingerprint":"abc3","duration":137,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Yellow Submarine", "title":"Love Me Do"},"weighting": 0}`, true)
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc5", "", 200, `{"fingerprint":"abc5","duration":137,"url":"http://example.org/track5","trackid":5,"tags":{"artist":"Panpipes Cover Band", "title":"Yellow Submarine","genre":"panpipes"},"weighting": 0}`, true)

	// Ensure the tracks which don't match haven't changed
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc4", "", 200, `{"fingerprint":"abc4","duration":7,"url":"http://example.org/track4","trackid":4,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"},"weighting": 0}`, true)

}

/**
 * Checks pagination when a query is used
 */
func TestBulkUpdatePaginationV2(test *testing.T) {
	clearData()

	// Create 45 Tracks
	for i := 1; i <= 45; i++ {
		id := strconv.Itoa(i)
		trackurl := "http://example.org/track/id" + id
		escapedTrackUrl := url.QueryEscape(trackurl)
		trackpath := fmt.Sprintf("/v2/tracks?url=%s", escapedTrackUrl)
		inputJson := `{"fingerprint": "abcde` + id + `", "duration": 350, "tags":{"title":"Yellow Submarine"}}`
		outputJson := `{"fingerprint": "abcde` + id + `", "duration": 350, "url": "` + trackurl + `", "trackid": ` + id + `, "tags": {"title":"Yellow Submarine"}, "weighting": 0}`
		makeRequest(test, "PUT", trackpath, inputJson, 200, outputJson, true)
		makeRequest(test, "PUT", "/v2/tracks/"+id+"/weighting", "4.3", 200, "4.3", false)
	}


	restartServer() // Clear Loganne counters etc
	request := basicRequest(test, "PATCH", "/v2/tracks?q=Yellow%20Submarine&page=2", `{"tags":{"title": "Blue Submarine"}}`)
	response := makeRawRequest(test, request, 200, `{"tracks":[{"fingerprint":"abcde21","duration":350,"url":"http://example.org/track/id21","trackid":21,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde22","duration":350,"url":"http://example.org/track/id22","trackid":22,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde23","duration":350,"url":"http://example.org/track/id23","trackid":23,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde24","duration":350,"url":"http://example.org/track/id24","trackid":24,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde25","duration":350,"url":"http://example.org/track/id25","trackid":25,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde26","duration":350,"url":"http://example.org/track/id26","trackid":26,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde27","duration":350,"url":"http://example.org/track/id27","trackid":27,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde28","duration":350,"url":"http://example.org/track/id28","trackid":28,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde29","duration":350,"url":"http://example.org/track/id29","trackid":29,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde30","duration":350,"url":"http://example.org/track/id30","trackid":30,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde31","duration":350,"url":"http://example.org/track/id31","trackid":31,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde32","duration":350,"url":"http://example.org/track/id32","trackid":32,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde33","duration":350,"url":"http://example.org/track/id33","trackid":33,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde34","duration":350,"url":"http://example.org/track/id34","trackid":34,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde35","duration":350,"url":"http://example.org/track/id35","trackid":35,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde36","duration":350,"url":"http://example.org/track/id36","trackid":36,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde37","duration":350,"url":"http://example.org/track/id37","trackid":37,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde38","duration":350,"url":"http://example.org/track/id38","trackid":38,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde39","duration":350,"url":"http://example.org/track/id39","trackid":39,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde40","duration":350,"url":"http://example.org/track/id40","trackid":40,"tags":{"title":"Blue Submarine"},"weighting":4.3}],"totalPages":3}`, true)
	checkResponseHeader(test, response, "Track-Action", "tracksUpdated")
	assertEqual(test, "Loganne call", "tracksUpdated", lastLoganneType)
	assertEqual(test, "Number of Loganne requests", 21, loganneRequestCount) // One request for each track changed, plus one for the bulk change

	// Test a track on the matching page is updated
	makeRequest(test, "GET", "/v2/tracks/27", "", 200, `{"fingerprint":"abcde27","duration":350,"url":"http://example.org/track/id27","trackid":27,"tags":{"title":"Blue Submarine"},"weighting": 4.3}`, true)

	// Test tracks before and after the page haven't been updated
	makeRequest(test, "GET", "/v2/tracks/20", "", 200, `{"fingerprint":"abcde20","duration":350,"url":"http://example.org/track/id20","trackid":20,"tags":{"title":"Yellow Submarine"},"weighting": 4.3}`, true)
	makeRequest(test, "GET", "/v2/tracks/41", "", 200, `{"fingerprint":"abcde41","duration":350,"url":"http://example.org/track/id41","trackid":41,"tags":{"title":"Yellow Submarine"},"weighting": 4.3}`, true)
}

/**
 * Checks that page=all will edit tracks on all pages
 */
func TestBulkUpdateNoPaginationV2(test *testing.T) {
	clearData()

	// Create 45 Tracks
	for i := 1; i <= 45; i++ {
		id := strconv.Itoa(i)
		trackurl := "http://example.org/track/id" + id
		escapedTrackUrl := url.QueryEscape(trackurl)
		trackpath := fmt.Sprintf("/v2/tracks?url=%s", escapedTrackUrl)
		inputJson := `{"fingerprint": "abcde` + id + `", "duration": 350, "tags":{"title":"Yellow Submarine"}}`
		outputJson := `{"fingerprint": "abcde` + id + `", "duration": 350, "url": "` + trackurl + `", "trackid": ` + id + `, "tags": {"title":"Yellow Submarine"}, "weighting": 0}`
		makeRequest(test, "PUT", trackpath, inputJson, 200, outputJson, true)
		makeRequest(test, "PUT", "/v2/tracks/"+id+"/weighting", "4.3", 200, "4.3", false)
	}


	restartServer() // Clear Loganne counters etc
	request := basicRequest(test, "PATCH", "/v2/tracks?q=Yellow%20Submarine&page=all", `{"tags":{"title": "Blue Submarine"}}`)
	response := makeRawRequest(test, request, 200, `{"tracks":[{"fingerprint":"abcde1","duration":350,"url":"http://example.org/track/id1","trackid":1,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde2","duration":350,"url":"http://example.org/track/id2","trackid":2,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde3","duration":350,"url":"http://example.org/track/id3","trackid":3,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde4","duration":350,"url":"http://example.org/track/id4","trackid":4,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde5","duration":350,"url":"http://example.org/track/id5","trackid":5,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde6","duration":350,"url":"http://example.org/track/id6","trackid":6,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde7","duration":350,"url":"http://example.org/track/id7","trackid":7,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde8","duration":350,"url":"http://example.org/track/id8","trackid":8,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde9","duration":350,"url":"http://example.org/track/id9","trackid":9,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde10","duration":350,"url":"http://example.org/track/id10","trackid":10,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde11","duration":350,"url":"http://example.org/track/id11","trackid":11,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde12","duration":350,"url":"http://example.org/track/id12","trackid":12,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde13","duration":350,"url":"http://example.org/track/id13","trackid":13,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde14","duration":350,"url":"http://example.org/track/id14","trackid":14,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde15","duration":350,"url":"http://example.org/track/id15","trackid":15,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde16","duration":350,"url":"http://example.org/track/id16","trackid":16,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde17","duration":350,"url":"http://example.org/track/id17","trackid":17,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde18","duration":350,"url":"http://example.org/track/id18","trackid":18,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde19","duration":350,"url":"http://example.org/track/id19","trackid":19,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde20","duration":350,"url":"http://example.org/track/id20","trackid":20,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde21","duration":350,"url":"http://example.org/track/id21","trackid":21,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde22","duration":350,"url":"http://example.org/track/id22","trackid":22,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde23","duration":350,"url":"http://example.org/track/id23","trackid":23,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde24","duration":350,"url":"http://example.org/track/id24","trackid":24,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde25","duration":350,"url":"http://example.org/track/id25","trackid":25,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde26","duration":350,"url":"http://example.org/track/id26","trackid":26,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde27","duration":350,"url":"http://example.org/track/id27","trackid":27,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde28","duration":350,"url":"http://example.org/track/id28","trackid":28,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde29","duration":350,"url":"http://example.org/track/id29","trackid":29,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde30","duration":350,"url":"http://example.org/track/id30","trackid":30,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde31","duration":350,"url":"http://example.org/track/id31","trackid":31,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde32","duration":350,"url":"http://example.org/track/id32","trackid":32,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde33","duration":350,"url":"http://example.org/track/id33","trackid":33,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde34","duration":350,"url":"http://example.org/track/id34","trackid":34,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde35","duration":350,"url":"http://example.org/track/id35","trackid":35,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde36","duration":350,"url":"http://example.org/track/id36","trackid":36,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde37","duration":350,"url":"http://example.org/track/id37","trackid":37,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde38","duration":350,"url":"http://example.org/track/id38","trackid":38,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde39","duration":350,"url":"http://example.org/track/id39","trackid":39,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde40","duration":350,"url":"http://example.org/track/id40","trackid":40,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde41","duration":350,"url":"http://example.org/track/id41","trackid":41,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde42","duration":350,"url":"http://example.org/track/id42","trackid":42,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde43","duration":350,"url":"http://example.org/track/id43","trackid":43,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde44","duration":350,"url":"http://example.org/track/id44","trackid":44,"tags":{"title":"Blue Submarine"},"weighting":4.3},{"fingerprint":"abcde45","duration":350,"url":"http://example.org/track/id45","trackid":45,"tags":{"title":"Blue Submarine"},"weighting":4.3}],"totalPages":3}`, true)
	checkResponseHeader(test, response, "Track-Action", "tracksUpdated")
	assertEqual(test, "Loganne call", "tracksUpdated", lastLoganneType)
	assertEqual(test, "Number of Loganne requests", 46, loganneRequestCount) // One request for each track changed, plus one for the bulk change

	// Test one track on each page is updated
	makeRequest(test, "GET", "/v2/tracks/20", "", 200, `{"fingerprint":"abcde20","duration":350,"url":"http://example.org/track/id20","trackid":20,"tags":{"title":"Blue Submarine"},"weighting": 4.3}`, true)
	makeRequest(test, "GET", "/v2/tracks/27", "", 200, `{"fingerprint":"abcde27","duration":350,"url":"http://example.org/track/id27","trackid":27,"tags":{"title":"Blue Submarine"},"weighting": 4.3}`, true)
	makeRequest(test, "GET", "/v2/tracks/41", "", 200, `{"fingerprint":"abcde41","duration":350,"url":"http://example.org/track/id41","trackid":41,"tags":{"title":"Blue Submarine"},"weighting": 4.3}`, true)
}


/**
 * Checks case where no tracks match query
 */
 func TestPatchTracksNoMatchV2(test *testing.T) {
	clearData()

	// Title is Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc1", `{"url":"http://example.org/track1", "duration": 7,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"}}`, 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"},"weighting": 0}`, true)
	// Title contains Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc2", `{"url":"http://example.org/track2", "duration": 7,"tags":{"artist":"The Ladybirds", "title":"Want to visit a Yellow Submarine"}}`, 200, `{"fingerprint":"abc2","duration":7,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"The Ladybirds", "title":"Want to visit a Yellow Submarine"},"weighting": 0}`, true)
	// Artist is Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc3", `{"url":"http://example.org/track3", "duration": 7,"tags":{"artist":"Yellow Submarine", "title":"Love Me Do"}}`, 200, `{"fingerprint":"abc3","duration":7,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Yellow Submarine", "title":"Love Me Do"},"weighting": 0}`, true)
	// No mention of Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc4", `{"url":"http://example.org/track4", "duration": 7,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"}}`, 200, `{"fingerprint":"abc4","duration":7,"url":"http://example.org/track4","trackid":4,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"},"weighting": 0}`, true)
	// Title is also Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc5", `{"url":"http://example.org/track5", "duration": 7,"tags":{"artist":"Panpipes Cover Band", "title":"Yellow Submarine", "genre": "panpipes"}}`, 200, `{"fingerprint":"abc5","duration":7,"url":"http://example.org/track5","trackid":5,"tags":{"artist":"Panpipes Cover Band", "title":"Yellow Submarine", "genre": "panpipes"},"weighting": 0}`, true)

	restartServer() // Clear Loganne counters etc
	request := basicRequest(test, "PATCH", "/v2/tracks?q=Green%20Submarine", `{"tags": {"genre":"Maritime Songs"}}`)
	response := makeRawRequest(test, request, 200, `{"tracks":[],"totalPages":0}`, true)
	checkResponseHeader(test, response, "Track-Action", "noChange")

	// No calls to Loganne should be made, as nothing was updated
	assertEqual(test, "Loganne call", "", lastLoganneType)
	assertEqual(test, "Number of Loganne requests", 0, loganneRequestCount)

	// Ensure none of the tracks have changed
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc1", "", 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"},"weighting": 0}`, true)
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc2", "", 200, `{"fingerprint":"abc2","duration":7,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"The Ladybirds", "title":"Want to visit a Yellow Submarine"},"weighting": 0}`, true)
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc3", "", 200, `{"fingerprint":"abc3","duration":7,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Yellow Submarine", "title":"Love Me Do"},"weighting": 0}`, true)
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc4", "", 200, `{"fingerprint":"abc4","duration":7,"url":"http://example.org/track4","trackid":4,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"},"weighting": 0}`, true)
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc5", "", 200, `{"fingerprint":"abc5","duration":7,"url":"http://example.org/track5","trackid":5,"tags":{"artist":"Panpipes Cover Band", "title":"Yellow Submarine","genre":"panpipes"},"weighting": 0}`, true)

}
/**
 * Checks case where tracks do match the query, but they all match the target state of the edit
 */
 func TestPatchTracksNoUpdateNeededV2(test *testing.T) {
	clearData()

	// Title is Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc1", `{"url":"http://example.org/track1", "duration": 7,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"}}`, 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"},"weighting": 0}`, true)
	// Title contains Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc2", `{"url":"http://example.org/track2", "duration": 7,"tags":{"artist":"The Ladybirds", "title":"Want to visit a Yellow Submarine"}}`, 200, `{"fingerprint":"abc2","duration":7,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"The Ladybirds", "title":"Want to visit a Yellow Submarine"},"weighting": 0}`, true)
	// Artist is Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc3", `{"url":"http://example.org/track3", "duration": 7,"tags":{"artist":"Yellow Submarine", "title":"Love Me Do"}}`, 200, `{"fingerprint":"abc3","duration":7,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Yellow Submarine", "title":"Love Me Do"},"weighting": 0}`, true)
	// No mention of Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc4", `{"url":"http://example.org/track4", "duration": 7,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"}}`, 200, `{"fingerprint":"abc4","duration":7,"url":"http://example.org/track4","trackid":4,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"},"weighting": 0}`, true)
	// Title is also Yellow Submarine
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc5", `{"url":"http://example.org/track5", "duration": 7,"tags":{"artist":"Panpipes Cover Band", "title":"Yellow Submarine", "genre": "panpipes"}}`, 200, `{"fingerprint":"abc5","duration":7,"url":"http://example.org/track5","trackid":5,"tags":{"artist":"Panpipes Cover Band", "title":"Yellow Submarine", "genre": "panpipes"},"weighting": 0}`, true)

	restartServer() // Clear Loganne counters etc
	request := basicRequest(test, "PATCH", "/v2/tracks?q=Panpipes%20Cover%20Band", `{"tags": {"genre":"panpipes"}}`)
	response := makeRawRequest(test, request, 200, `{"tracks":[],"totalPages":1}`, true)
	checkResponseHeader(test, response, "Track-Action", "noChange")

	// No calls to Loganne should be made, as nothing was updated
	assertEqual(test, "Loganne call", "", lastLoganneType)
	assertEqual(test, "Number of Loganne requests", 0, loganneRequestCount)

	// Ensure none of the tracks have changed
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc1", "", 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"},"weighting": 0}`, true)
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc2", "", 200, `{"fingerprint":"abc2","duration":7,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"The Ladybirds", "title":"Want to visit a Yellow Submarine"},"weighting": 0}`, true)
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc3", "", 200, `{"fingerprint":"abc3","duration":7,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Yellow Submarine", "title":"Love Me Do"},"weighting": 0}`, true)
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc4", "", 200, `{"fingerprint":"abc4","duration":7,"url":"http://example.org/track4","trackid":4,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"},"weighting": 0}`, true)
	makeRequest(test, "GET", "/v2/tracks?fingerprint=abc5", "", 200, `{"fingerprint":"abc5","duration":7,"url":"http://example.org/track5","trackid":5,"tags":{"artist":"Panpipes Cover Band", "title":"Yellow Submarine","genre":"panpipes"},"weighting": 0}`, true)

}