package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

type loganneCapture struct {
	mu  sync.Mutex
	req *http.Request
	body string
	err  error
}

func (c *loganneCapture) get() (*http.Request, string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.req, c.body, c.err
}

func newMockLoganneServer() (*httptest.Server, *loganneCapture) {
	cap := &loganneCapture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		cap.mu.Lock()
		cap.req = r
		cap.body = string(body)
		cap.err = err
		cap.mu.Unlock()
		fmt.Fprint(w, "Received")
	}))
	return server, cap
}

func TestLoganneEvent(test *testing.T) {
	server, cap := newMockLoganneServer()
	defer server.Close()

	loganne := Loganne{
		endpoint:                   server.URL + "/events",
		source:                     "metadata_api_test",
		mediaMetadataManagerOrigin: "http://localhost:8020",
	}
	track := Track{
		ID:          17,
		URL:         "http://example.com/track-1.mp3",
		Fingerprint: "abc",
		Duration:    137,
		Weighting:   9,
		Tags: TagList{
			Tag{PredicateID: "artist", Value: "Altan"},
			Tag{PredicateID: "album", Value: "Harvest Storm"},
		},
	}
	loganne.post("testEvent", "This event is from the test", track, Track{})

	latestRequest, latestRequestBody, latestRequestError := cap.get()
	assertEqual(test, "Loganne request made to wrong path", "/events", latestRequest.URL.Path)
	assertEqual(test, "Loganne request wasn't POST request", "POST", latestRequest.Method)
	assertNoError(test, "Failed to get request body", latestRequestError)
	assertEqual(test, "Unexpected request body", `{"humanReadable":"This event is from the test","source":"metadata_api_test","track":{"fingerprint":"abc","duration":137,"url":"http://example.com/track-1.mp3","id":17,"tags":{"album":[{"name":"Harvest Storm"}],"artist":[{"name":"Altan"}]},"weighting":9},"type":"testEvent","url":"http://localhost:8020/tracks/17"}`, latestRequestBody)
}

func TestLoganneDeleteEvent(test *testing.T) {
	server, cap := newMockLoganneServer()
	defer server.Close()

	loganne := Loganne{
		endpoint:                   server.URL + "/events",
		source:                     "metadata_api_test",
		mediaMetadataManagerOrigin: "http://localhost:8020",
	}
	track := Track{
		ID:          18,
		URL:         "http://example.com/track-1.mp3",
		Fingerprint: "abc",
		Duration:    137,
		Weighting:   9,
		Tags: TagList{
			Tag{PredicateID: "artist", Value: "Altan"},
			Tag{PredicateID: "album", Value: "Harvest Storm"},
		},
	}

	// Delete event passes empty track for the updated track, then existing track
	loganne.post("deleteEvent", "This event is from the delete test", Track{}, track)

	latestRequest, latestRequestBody, latestRequestError := cap.get()
	assertEqual(test, "Loganne request made to wrong path", "/events", latestRequest.URL.Path)
	assertEqual(test, "Loganne request wasn't POST request", "POST", latestRequest.Method)
	assertNoError(test, "Failed to get request body", latestRequestError)
	assertEqual(test, "Unexpected request body", `{"humanReadable":"This event is from the delete test","source":"metadata_api_test","track":{"fingerprint":"abc","duration":137,"url":"http://example.com/track-1.mp3","id":18,"tags":{"album":[{"name":"Harvest Storm"}],"artist":[{"name":"Altan"}]},"weighting":9},"type":"deleteEvent","url":"http://localhost:8020/tracks/18"}`, latestRequestBody)
}

/**
 * Test loganne for an event which modifies multiple tracks
 **/
func TestBulkEvent(test *testing.T) {
	server, cap := newMockLoganneServer()
	defer server.Close()

	loganne := Loganne{
		endpoint: server.URL + "/events",
		source:   "metadata_api_test",
	}
	loganne.post("bulkTestEvent", "This event is from the bulk test", Track{}, Track{})

	latestRequest, latestRequestBody, latestRequestError := cap.get()
	assertEqual(test, "Loganne request made to wrong path", "/events", latestRequest.URL.Path)
	assertEqual(test, "Loganne request wasn't POST request", "POST", latestRequest.Method)
	assertNoError(test, "Failed to get request body", latestRequestError)
	assertEqual(test, "Unexpected request body", `{"humanReadable":"This event is from the bulk test","source":"metadata_api_test","type":"bulkTestEvent"}`, latestRequestBody)
}
