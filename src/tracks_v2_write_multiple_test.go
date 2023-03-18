package main
import (
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

	request := basicRequest(test, "PATCH", "/v2/tracks?p.title=Yellow%20Submarine", `{"tags": {"genre":"Maritime Songs"}}`)
	response := makeRawRequest(test, request, 200, `{"tracks":[{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles","genre":"Maritime Songs","title":"Yellow Submarine"},"weighting":0},{"fingerprint":"abc5","duration":7,"url":"http://example.org/track5","trackid":5,"tags":{"artist":"Panpipes Cover Band","genre":"Maritime Songs","title":"Yellow Submarine"},"weighting":0}],"totalPages":1}`, true)
	checkResponseHeader(test, response, "Track-Action", "tracksUpdated")
	assertEqual(test, "Loganne call", "tracksUpdated", lastLoganneType)

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

	request := basicRequest(test, "PATCH", "/v2/tracks?p.title=Yellow%20Submarine", `{"tags": {"genre":"Maritime Songs"}}`)
	request.Header.Add("If-None-Match", "*")
	response := makeRawRequest(test, request, 200, `{"tracks":[{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles","genre":"Maritime Songs","title":"Yellow Submarine"},"weighting":0},{"fingerprint":"abc5","duration":7,"url":"http://example.org/track5","trackid":5,"tags":{"artist":"Panpipes Cover Band","genre":"panpipes","title":"Yellow Submarine"},"weighting":0}],"totalPages":1}`, true)
	checkResponseHeader(test, response, "Track-Action", "tracksUpdated")
	assertEqual(test, "Loganne call", "tracksUpdated", lastLoganneType)

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

	request := basicRequest(test, "PATCH", "/v2/tracks?p.title=Yellow%20Submarine&p.artist=Panpipes%20Cover%20Band", `{"tags": {"genre":"Maritime Songs"}}`)
	response := makeRawRequest(test, request, 200, `{"tracks":[{"fingerprint":"abc5","duration":7,"url":"http://example.org/track5","trackid":5,"tags":{"artist":"Panpipes Cover Band","genre":"Maritime Songs","title":"Yellow Submarine"},"weighting":0}],"totalPages":1}`, true)
	checkResponseHeader(test, response, "Track-Action", "tracksUpdated")
	assertEqual(test, "Loganne call", "tracksUpdated", lastLoganneType)

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

	request := basicRequest(test, "PATCH", "/v2/tracks?q=Yellow%20Submarine", `{"tags": {"genre":"Maritime Songs"}}`)
	response := makeRawRequest(test, request, 200, `{"tracks":[{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles","genre":"Maritime Songs","title":"Yellow Submarine"},"weighting":0},{"fingerprint":"abc2","duration":7,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"The Ladybirds","genre":"Maritime Songs","title":"Want to visit a Yellow Submarine"},"weighting":0},{"fingerprint":"abc3","duration":7,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Yellow Submarine","genre":"Maritime Songs","title":"Love Me Do"},"weighting":0},{"fingerprint":"abc5","duration":7,"url":"http://example.org/track5","trackid":5,"tags":{"artist":"Panpipes Cover Band","genre":"Maritime Songs","title":"Yellow Submarine"},"weighting":0}],"totalPages":1}`, true)
	checkResponseHeader(test, response, "Track-Action", "tracksUpdated")
	assertEqual(test, "Loganne call", "tracksUpdated", lastLoganneType)

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
 * Checks that attempt to update duration returns a resonable error
 */
func TestCantBulkUpdateDuration(test *testing.T) {
	makeRequest(test, "PATCH", "/v2/tracks?q=Yellow", `{"duration": 137}`, 404, "Bulk update of duration not supported\n", false)
}