package main

import (
    "fmt"
    "net/http"
    "net/http/httptest"
    "net/url"
    "strings"
    "testing"
    "io/ioutil"
    "encoding/json"
    "reflect"
    "os"
)

var server *httptest.Server

/**
 * Creats a test http server, using the FrontController under test
 * Stores the URL of the server in a global variable
 */
func init() {
	os.Remove("testrouting.sqlite")
    restartServer()
}
func TestMain(m *testing.M) {
	os.Remove("testrouting.sqlite")
	restartServer()
	result := m.Run()
	os.Remove("testrouting.sqlite")
	os.Exit(result)
}

func restartServer() {
	if server != nil {
		server.Close()
	}
	store, _ := DBInit("testrouting.sqlite")
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
 * Makes a http request and compares the response to some expected values
 *
 * (Probably not the most go-eque way to do this, but it works for me)
 */
func makeRequest(t *testing.T, method string, path string, requestBody string, expectedResponseCode int, expectedResponseBody string, expectJSON bool) {
    reader := strings.NewReader(requestBody)
    url := server.URL + path
    request, err := http.NewRequest(method, url, reader)
    if err != nil {
        t.Error(err)
    }

    response, err := http.DefaultClient.Do(request)
    if err != nil {
        t.Error(err)
    }

    if response.StatusCode != expectedResponseCode {
        t.Errorf("Got response code %d, expected %d", response.StatusCode, expectedResponseCode)
    }
    responseData,err := ioutil.ReadAll(response.Body)
	if err != nil {
	    t.Error(err)
	}
	actualResponseBody := string(responseData)
	if (expectJSON) {
		if (response.Header.Get("content-type") != "application/json") {
			t.Errorf("Expected JSON Content Type, received: \"%s\"", request.Header.Get("Content-type"))
		}
		isEqual, err := AreEqualJSON(actualResponseBody, expectedResponseBody)
		if err != nil {
		    t.Error(err)
		}
		if (!isEqual) {
			t.Errorf("Unexpected JSON body: %s, expected: %s", actualResponseBody, expectedResponseBody)
		}
	} else {
		if (actualResponseBody != expectedResponseBody) {
			t.Errorf("Unexpected body: \"%s\", expected: \"%s\"", actualResponseBody, expectedResponseBody)
		}
	}
}

/**
 * Checks whether a track can be edited based on its url and retrieved later
 */
func TestCanEditTrack(test *testing.T) {
	trackurl := "http://example.org/track/1256"
	escapedTrackUrl := url.QueryEscape(trackurl)
	inputJson := `{"fingerprint": "aoecu1234", "duration": 300}`
	outputJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/1256"}`
	path := fmt.Sprintf("/tracks?url=%s", escapedTrackUrl)
	makeRequest(test, "GET", path, "", 404, "Track Not Found\n", false)
	makeRequest(test, "PUT", path, inputJson, 200, outputJson, true)
	makeRequest(test, "GET", path, "", 200, outputJson, true)
	restartServer()
	makeRequest(test, "GET", path, "", 200, outputJson, true)
}