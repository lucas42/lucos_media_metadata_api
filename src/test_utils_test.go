package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

var server *httptest.Server
var lastLoganneType string
var lastLoganneMessage string
var lastLoganneTrack Track
var lastLoganneExistingTrack Track
var loganneRequestCount int

type MockLoganne struct {}
func (mock MockLoganne) post(eventType string, humanReadable string, track Track, existingTrack Track) {
	lastLoganneType = eventType
	lastLoganneMessage = humanReadable
	lastLoganneTrack = track
	lastLoganneExistingTrack = existingTrack
	loganneRequestCount++
}

func clearData() {
	os.Remove("testrouting.sqlite")
	restartServer()
}
func TestMain(m *testing.M) {
	log.SetOutput(ioutil.Discard)
	clearData()
	result := m.Run()
	os.Remove("testrouting.sqlite")
	os.Exit(result)
}

/**
 * Creates a test http server, using the FrontController under test
 * Stores the URL of the server in a global variable
 */
func restartServer() {
	if server != nil {
		server.Close()
	}
	lastLoganneType = ""
	lastLoganneMessage = ""
	lastLoganneTrack = Track{}
	lastLoganneExistingTrack = Track{}
	loganneRequestCount = 0
	store := DBInit("testrouting.sqlite", MockLoganne{})
	server = httptest.NewServer(FrontController(store))
}

/**
 * Checks whether 2 JSON strings are equivalent
 *
 * Source: https://gist.github.com/turtlemonvh/e4f7404e28387fadb8ad275a99596f67
 */
func AreEqualJSON(s1, s2 string) (bool, error) {
	var o1 interface{}
	var o2 interface{}

	// The `added` tag can very based on time of request, so ignore from this check
	re := regexp.MustCompile(`"added": *".*?",?`)
	s1 = re.ReplaceAllString(s1, "")

	var err error
	err = json.Unmarshal([]byte(s1), &o1)
	if err != nil {
		return false, fmt.Errorf("Error mashalling string 1 :: %s", err.Error())
	}
	err = json.Unmarshal([]byte(s2), &o2)
	if err != nil {
		return false, fmt.Errorf("Error mashalling string 2 :: %s", err.Error())
	}

	return reflect.DeepEqual(o1, o2), nil
}

/**
 * Constructs a http request and compares the response to some expected values
 *
 */
func makeRequest(t *testing.T, method string, path string, requestBody string, expectedResponseCode int, expectedResponseBody string, expectJSON bool) {
	request := basicRequest(t, method, path, requestBody)
	makeRawRequest(t, request, expectedResponseCode, expectedResponseBody, expectJSON)
}

/**
 * Constructs a http request against the test server
 *
 */
func basicRequest(t *testing.T, method string, path string, requestBody string) (request *http.Request) {
	reader := strings.NewReader(requestBody)
	url := server.URL + path
	var err error
	request, err = http.NewRequest(method, url, reader)
	if err != nil {
		t.Error(err)
	}
	return
}

/**
 * Makes a http request and compares the response to some expected values
 *
 * (Probably not the most go-eque way to do this, but it works for me)
 */
func makeRawRequest(t *testing.T, request *http.Request, expectedResponseCode int, expectedResponseBody string, expectJSON bool) (response *http.Response) {
	url := request.URL.String()
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Error(err)
	}
	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		t.Error(err)
	}
	actualResponseBody := string(responseData)

	if response.StatusCode >= 500 {
		t.Errorf("Error code %d returned for %s, message: %s", response.StatusCode, url, actualResponseBody)
		return
	}

	if response.StatusCode != expectedResponseCode {
		t.Errorf("Got response code %d, expected %d for %s", response.StatusCode, expectedResponseCode, url)
	}
	if expectJSON {
		if !strings.HasPrefix(response.Header.Get("content-type"), "application/json;") {
			t.Errorf("Expected JSON Content Type, received: \"%s\"", request.Header.Get("Content-type"))
		}
		isEqual, err := AreEqualJSON(actualResponseBody, expectedResponseBody)
		if err != nil {
			t.Errorf("Invalid JSON body: %s, expected: %s from %s", actualResponseBody, expectedResponseBody, url)
		} else if !isEqual {
			t.Errorf("Unexpected JSON body: %s, expected: %s", actualResponseBody, expectedResponseBody)
		}
	} else {
		if actualResponseBody != expectedResponseBody {
			t.Errorf("Unexpected body: \"%s\", expected: \"%s\"", actualResponseBody, expectedResponseBody)
		}
	}
	return
}

/**
 * Constructs a http request with a method which shouldn't be allowed
 *
 */
func makeRequestWithUnallowedMethod(t *testing.T, path string, unallowedMethod string, allowedMethods []string) {
	request := basicRequest(t, unallowedMethod, path, "")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Error(err)
	}
	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		t.Error(err)
	}
	actualResponseBody := string(responseData)
	if response.StatusCode != 405 {
		t.Errorf("Got response code %d, expected %d for %s", response.StatusCode, 405, path)
	}
	if !strings.HasPrefix(actualResponseBody, "Method Not Allowed") {
		t.Errorf("Repsonse body for %s doesn't begin with \"Method Not Allowed\"", path)
	}
	for _, method := range allowedMethods {
		if !strings.Contains(response.Header.Get("allow"), method) {
			t.Errorf("Allow header doesn't include method %s for %s", method, path)
		}
	}
}

func checkResponseHeader(t *testing.T, response *http.Response, headerName string, expectedValue string) {
	actualValue := response.Header.Get(headerName)
	if actualValue == "" {
		t.Errorf("Response header \"%s\" missing", headerName)
		return
	}
	assertEqual(t, "Incorrect response header for \""+headerName+"\"", expectedValue, actualValue)
}

/**
 * Constructs a http request which should do a redirect
 *
 */
func checkRedirect(t *testing.T, initialPath string, expectedPath string) {
	initialURL := server.URL + initialPath
	expectedDestination := server.URL + expectedPath
	response, err := http.Get(initialURL)
	if err != nil {
		t.Error(err)
	}
	actualDestination := response.Request.URL.String()
	if actualDestination != expectedDestination {
		t.Errorf("Unexpected redirect destination: \"%s\", expected: \"%s\", from \"%s\"", actualDestination, expectedDestination, initialURL)
	}
}

