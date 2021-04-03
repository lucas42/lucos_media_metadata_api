package main

import (
	"testing"
)

/**
 * Checks whether the correct tracks are returned from search
 */
func TestSimpleSearch(test *testing.T) {
	clearData()

	// Mentions blue in the title
	makeRequest(test, "PUT", "/tracks?fingerprint=abc1", `{"url":"http://example.org/track1", "duration": 7,"tags":{"artist":"Eiffel 65", "title":"I'm blue"}}`, 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"Eiffel 65", "title":"I'm blue"},"weighting": 0}`, true)
	// Artist is Blue (with a captial B)
	makeRequest(test, "PUT", "/tracks?fingerprint=abc2", `{"url":"http://example.org/track2", "duration": 7,"tags":{"artist":"Blue", "title":"I can"}}`, 200, `{"fingerprint":"abc2","duration":7,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"Blue", "title":"I can"},"weighting": 0}`, true)
	// No mention of blue
	makeRequest(test, "PUT", "/tracks?fingerprint=abc3", `{"url":"http://example.org/track3", "duration": 7,"tags":{"artist":"Coldplay", "title":"Yellow"}}`, 200, `{"fingerprint":"abc3","duration":7,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Coldplay", "title":"Yellow"},"weighting": 0}`, true)
	// Genre is blues
	makeRequest(test, "PUT", "/tracks?fingerprint=abc4", `{"url":"http://example.org/track4", "duration": 7,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"}}`, 200, `{"fingerprint":"abc4","duration":7,"url":"http://example.org/track4","trackid":4,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"},"weighting": 0}`, true)

	makeRequest(test, "GET", "/search?q=blue", "", 200, `{"tracks":[
		{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"Eiffel 65", "title":"I'm blue"},"weighting": 0},
		{"fingerprint":"abc2","duration":7,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"Blue", "title":"I can"},"weighting": 0},
		{"fingerprint":"abc4","duration":7,"url":"http://example.org/track4","trackid":4,"tags":{"artist":"Robert Johnson", "title":"Sweet Home Chicago", "genre": "blues"},"weighting": 0}
	]}`, true)
}

func TestInvalidRequestsToSearch(test *testing.T) {
	makeRequestWithUnallowedMethod(test, "/search?q=blue", "POST", []string{"GET"})
	makeRequest(test, "GET", "/search", "", 400, "No query given\n", false)
	makeRequest(test, "GET", "/search?q=", "", 400, "No query given\n", false)
}