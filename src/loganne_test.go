package main

import (
	"testing"
	"net/http"
	"fmt"
	"io/ioutil"
)

var latestRequest *http.Request
var latestRequestBody string
var latestRequestError error
func mockLoganneServer() {
    http.HandleFunc("/", mockLoganneEvent)
    http.ListenAndServe(":7999", nil)
}

func mockLoganneEvent(w http.ResponseWriter, request *http.Request) {
	latestRequest = request
	body, err := ioutil.ReadAll(latestRequest.Body)
	latestRequestBody = string(body)
	latestRequestError = err
    fmt.Fprint(w, "Received")
}

func TestLoganneEvent(test *testing.T) {
	go mockLoganneServer()
	loganne := Loganne{
		host: "http://localhost:7999",
		source: "metadata_api_test",
	}
	track := Track{
		ID: 17,
		URL: "http://example.com/track-1.mp3",
		Fingerprint: "abc",
		Duration: 137,
		Weighting: 9,
		Tags: map[string]string {
			"artist": "Altan",
			"album": "Harvest Storm",
		},
	}
	loganne.post("testEvent", "This event is from the test", track, Track{})

	assertEqual(test, "Loganne request made to wrong path", "/events", latestRequest.URL.Path)
	assertEqual(test,"Loganne request wasn't POST request", "POST", latestRequest.Method)

	assertNoError(test, "Failed to get request body", latestRequestError)
	assertEqual(test, "Unexpected request body", `{"humanReadable":"This event is from the test","source":"metadata_api_test","track":{"fingerprint":"abc","duration":137,"url":"http://example.com/track-1.mp3","trackid":17,"tags":{"album":"Harvest Storm","artist":"Altan"},"weighting":9},"type":"testEvent"}`, latestRequestBody)
}
func TestLoganneDeleteEvent(test *testing.T) {
	loganne := Loganne{
		host: "http://localhost:7999",
		source: "metadata_api_test",
	}
	track := Track{
		ID: 18,
		URL: "http://example.com/track-1.mp3",
		Fingerprint: "abc",
		Duration: 137,
		Weighting: 9,
		Tags: map[string]string {
			"artist": "Altan",
			"album": "Harvest Storm",
		},
	}

	// Delete event passes empty track for the updated track, then existing track
	loganne.post("deleteEvent", "This event is from the test", Track{}, track)

	assertEqual(test, "Loganne request made to wrong path", "/events", latestRequest.URL.Path)
	assertEqual(test,"Loganne request wasn't POST request", "POST", latestRequest.Method)

	assertNoError(test, "Failed to get request body", latestRequestError)
	assertEqual(test, "Unexpected request body", `{"humanReadable":"This event is from the test","source":"metadata_api_test","track":{"fingerprint":"abc","duration":137,"url":"http://example.com/track-1.mp3","trackid":18,"tags":{"album":"Harvest Storm","artist":"Altan"},"weighting":9},"type":"deleteEvent"}`, latestRequestBody)
}

func assertEqual(test *testing.T, message string, expected interface{}, actual interface{}) {
	if expected != actual {
		test.Errorf("%s. Expected: %s, Actual: %s", message, expected, actual)
	}
}
func assertNoError(test *testing.T, message string, err error) {
	if err != nil {
		test.Errorf("%s Error message: %s", message, err)
	}
}