package main

import (
	"testing"
)

/**
 * Checks whether a global value can be set and read
 */
func TestGlobals(test *testing.T) {
	clearData()
	path := "/globals/isXmas"
	makeRequest(test, "GET", path, "", 404, "Global Variable Not Found\n", false)
	makeRequest(test, "PUT", path, "yes", 200, "yes", false)
	makeRequest(test, "GET", path, "", 200, "yes", false)
	restartServer()
	makeRequest(test, "GET", path, "", 200, "yes", false)
	makeRequest(test, "PUT", path, "notyet", 200, "notyet", false)
	makeRequest(test, "GET", path, "", 200, "notyet", false)
	makeRequestWithUnallowedMethod(test, path, "POST", []string{"PUT", "GET"})
}

/**
 * Checks whether a list of all globals is returned from /globals
 */
func TestGetAllGlobals(test *testing.T) {
	clearData()
	makeRequest(test, "PUT", "/globals/isXmas", "yes", 200, "yes", false)
	makeRequest(test, "PUT", "/globals/recent_errors", "17", 200, "17", false)
	makeRequest(test, "PUT", "/globals/weather", "sunny", 200, "sunny", false)
	makeRequest(test, "GET", "/globals", "", 200, `{"isXmas":"yes","recent_errors":"17","weather":"sunny"}`, true)
	makeRequestWithUnallowedMethod(test, "/globals?abc", "PUT", []string{"GET"})
}
