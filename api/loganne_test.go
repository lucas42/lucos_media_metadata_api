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
		endpoint: "http://localhost:7999/events",
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
	assertEqual(test, "Unexpected request body", `{"humanReadable":"This event is from the test","source":"metadata_api_test","track":{"fingerprint":"abc","duration":137,"url":"http://example.com/track-1.mp3","trackid":17,"tags":{"album":"Harvest Storm","artist":"Altan"},"weighting":9},"type":"testEvent","url":"https://media-metadata.l42.eu/tracks/17"}`, latestRequestBody)
}
func TestLoganneDeleteEvent(test *testing.T) {
	loganne := Loganne{
		endpoint: "http://localhost:7999/events",
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
	loganne.post("deleteEvent", "This event is from the delete test", Track{}, track)

	assertEqual(test, "Loganne request made to wrong path", "/events", latestRequest.URL.Path)
	assertEqual(test,"Loganne request wasn't POST request", "POST", latestRequest.Method)

	assertNoError(test, "Failed to get request body", latestRequestError)
	assertEqual(test, "Unexpected request body", `{"humanReadable":"This event is from the delete test","source":"metadata_api_test","track":{"fingerprint":"abc","duration":137,"url":"http://example.com/track-1.mp3","trackid":18,"tags":{"album":"Harvest Storm","artist":"Altan"},"weighting":9},"type":"deleteEvent"}`, latestRequestBody)
}

/**
 * Test loganne for an event which modifies multiple tracks
 **/
func TestBulkEvent(test *testing.T) {
	loganne := Loganne{
		endpoint: "http://localhost:7999/events",
		source: "metadata_api_test",
	}
	loganne.post("bulkTestEvent", "This event is from the bulk test", Track{}, Track{})

	assertEqual(test, "Loganne request made to wrong path", "/events", latestRequest.URL.Path)
	assertEqual(test,"Loganne request wasn't POST request", "POST", latestRequest.Method)

	assertNoError(test, "Failed to get request body", latestRequestError)
	assertEqual(test, "Unexpected request body", `{"humanReadable":"This event is from the bulk test","source":"metadata_api_test","type":"bulkTestEvent"}`, latestRequestBody)
}
