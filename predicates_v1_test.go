package main

import (
	"testing"
)

/**
 * Checks whether new predicates can be added
 */
func TestAddPredicate(test *testing.T) {
	clearData()
	allpath := "/predicates/"
	path1 := "/predicates/artist"
	inputJson := `{}`
	output1Json := `{"id":"artist"}`
	list1Json := `[{"id":"artist"}]`
	makeRequest(test, "GET", path1, "", 404, "Predicate Not Found\n", false)
	makeRequest(test, "PUT", path1, inputJson, 200, output1Json, true)
	makeRequest(test, "GET", path1, "", 200, output1Json, true)
	makeRequest(test, "GET", allpath, "", 200, list1Json, true)
	restartServer()
	makeRequest(test, "GET", path1, "", 200, output1Json, true)
	makeRequestWithUnallowedMethod(test, allpath, "PUT", []string{"GET"})
	makeRequestWithUnallowedMethod(test, path1, "POST", []string{"PUT", "GET"})
}
