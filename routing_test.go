

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
)

var (
    serverUrl string
    fingerprintUrl string
)

func init() {
    server := httptest.NewServer(routing())
    serverUrl = server.URL
}

// Source: https://gist.github.com/turtlemonvh/e4f7404e28387fadb8ad275a99596f67
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

func makeRequest(t *testing.T, method string, url string, requestBody string, expectedResponseCode int, expectedResponseBody string, expectJSON bool) {
    reader := strings.NewReader(requestBody)
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

func TestRootFingerprint(t *testing.T) {
	url := fmt.Sprintf("%s/fingerprint/", serverUrl)
    makeRequest(t, "GET", url, "", 200, "Hello fingerprint! ", false)
}
func TestAddFingerprint(t *testing.T) {
	url := fmt.Sprintf("%s/fingerprint/abc123", serverUrl)
    fingerprintJson := `{"fingerprint": "123456"}`
    makeRequest(t, "PUT", url, fingerprintJson, 200, "PUT not done yet abc123", false)
}
func TestPostFingerprint(t *testing.T) {
	url := fmt.Sprintf("%s/fingerprint/abc245", serverUrl)
    fingerprintJson := `{"fingerprint": "123456"}`
    makeRequest(t, "POST", url, fingerprintJson, 405, "Method Not Allowed, must be one of: GET, PUT", false)
}
func TestCanEditTrack(test *testing.T) {
	trackurl := "http://example.org/track/1256"
	escapedTrackUrl := url.QueryEscape(trackurl)
	inputJson := `{"fingerprint": "aoecu1234", "duration": 300}`
	outputJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/1256"}`
	url := fmt.Sprintf("%s/tracks?url=%s", serverUrl, escapedTrackUrl)
	makeRequest(test, "GET", url, "", 404, "Track Not Found\n", false)
	makeRequest(test, "PUT", url, inputJson, 200, outputJson, true)
	makeRequest(test, "GET", url, "", 200, outputJson, true)
}