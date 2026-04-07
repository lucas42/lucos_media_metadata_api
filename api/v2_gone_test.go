package main

import (
	"testing"
)

/**
 * Checks that all v2 track and collection endpoints return a 410 Gone HTTP Response
 */
func TestV2EndpointsReturnGone(test *testing.T) {
	clearData()
	errorMessage := "Version 2 of the API is no longer available.  Find version 3 at /v3/tracks\n"
	makeRequest(test, "GET", "/v2/tracks", "", 410, errorMessage, false)
	makeRequest(test, "GET", "/v2/tracks/1", "", 410, errorMessage, false)
	makeRequest(test, "PUT", "/v2/tracks/1", "", 410, errorMessage, false)
	makeRequest(test, "GET", "/v2/collections", "", 410, errorMessage, false)
	makeRequest(test, "GET", "/v2/collections/basic", "", 410, errorMessage, false)
}
