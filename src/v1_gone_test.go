package main

import (
	"io/ioutil"
	"log"
	"testing"
)

/**
 * Checks that all v1 endpoints return a 410 Gone HTTP Response
 */
func TestV1EndpointsReturnGone(test *testing.T) {
	log.SetOutput(ioutil.Discard)
	clearData()
	errorMessage := "Version 1 of the API is no longer available.  Find version 2 at /v2/tracks\n"
	makeRequest(test, "GET", "/tracks", "", 410, errorMessage, false)
	makeRequest(test, "GET", "/tracks/3", "", 410, errorMessage, false)
	makeRequest(test, "GET", "/tracks", "", 410, errorMessage, false)
	makeRequest(test, "GET", "/globals", "", 410, errorMessage, false)
	makeRequest(test, "GET", "/globals/", "", 410, errorMessage, false)
	makeRequest(test, "GET", "/predicates/", "", 410, errorMessage, false)
	makeRequest(test, "GET", "/tags/4", "", 410, errorMessage, false)
	makeRequest(test, "GET", "/search?q=test", "", 410, errorMessage, false)
}