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
func clearData() {
	os.Remove("testrouting.sqlite")
    restartServer()
}
func TestMain(m *testing.M) {
	clearData()
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
    responseData,err := ioutil.ReadAll(response.Body)
	if err != nil {
	    t.Error(err)
	}
	actualResponseBody := string(responseData)

	if (response.StatusCode >= 500) {
		t.Errorf("Error code %d returned for %s, message: %s", response.StatusCode, url, actualResponseBody)
		return
	}

    if response.StatusCode != expectedResponseCode {
        t.Errorf("Got response code %d, expected %d for %s", response.StatusCode, expectedResponseCode, url)
    }
	if (expectJSON) {
		if (response.Header.Get("content-type") != "application/json") {
			t.Errorf("Expected JSON Content Type, received: \"%s\"", request.Header.Get("Content-type"))
		}
		isEqual, err := AreEqualJSON(actualResponseBody, expectedResponseBody)
		if err != nil {
			t.Errorf("Invalid JSON body: %s, expected: %s", actualResponseBody, expectedResponseBody)
		} else if (!isEqual) {
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
	clearData()
	trackurl := "http://example.org/track/1256"
	escapedTrackUrl := url.QueryEscape(trackurl)
	inputJson := `{"fingerprint": "aoecu1234", "duration": 300}`
	outputJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/1256", "trackid": 1}`
	path := fmt.Sprintf("/tracks?url=%s", escapedTrackUrl)
	makeRequest(test, "GET", path, "", 404, "Track Not Found\n", false)
	makeRequest(test, "PUT", path, inputJson, 200, outputJson, true)
	makeRequest(test, "GET", path, "", 200, outputJson, true)
	restartServer()
	makeRequest(test, "GET", path, "", 200, outputJson, true)
}
/**
 * Checks whether a track can have its duration updated
 */
func TestCanUpdateDuration(test *testing.T) {
	clearData()
	trackurl := "http://example.org/track/333"
	escapedTrackUrl := url.QueryEscape(trackurl)
	inputAJson := `{"fingerprint": "aoecu1234", "duration": 300}`
	inputBJson := `{"fingerprint": "aoecu1234", "duration": 150}`
	outputAJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/333", "trackid": 1}`
	outputBJson := `{"fingerprint": "aoecu1234", "duration": 150, "url": "http://example.org/track/333", "trackid": 1}`
	path := fmt.Sprintf("/tracks?url=%s", escapedTrackUrl)
	makeRequest(test, "GET", path, "", 404, "Track Not Found\n", false)
	makeRequest(test, "PUT", path, inputAJson, 200, outputAJson, true)
	makeRequest(test, "PUT", path, inputBJson, 200, outputBJson, true)
	makeRequest(test, "GET", path, "", 200, outputBJson, true)
}

/**
 * Checks whether a track can be updated using its trackid
 */
func TestCanUpdateById(test *testing.T) {
	clearData()
	track1url := "http://example.org/track/333"
	escapedTrack1Url := url.QueryEscape(track1url)
	track2url := "http://example.org/track/334"
	escapedTrack2Url := url.QueryEscape(track2url)
	inputAJson := `{"fingerprint": "aoecu1234", "duration": 300}`
	inputBJson := `{"fingerprint": "76543", "duration": 150, "url": "http://example.org/track/334"}`
	outputAJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/333", "trackid": 1}`
	outputBJson := `{"fingerprint": "76543", "duration": 150, "url": "http://example.org/track/334", "trackid": 1}`
	path1 := fmt.Sprintf("/tracks?url=%s", escapedTrack1Url)	
	path2 := fmt.Sprintf("/tracks?url=%s", escapedTrack2Url)
	denormpath := "/tracks/denorm/1"
	makeRequest(test, "GET", path1, "", 404, "Track Not Found\n", false)
	makeRequest(test, "GET", denormpath, "", 404, "Track Not Found\n", false)
	makeRequest(test, "PUT", path1, inputAJson, 200, outputAJson, true)
	makeRequest(test, "GET", path1, "", 200, outputAJson, true)
	makeRequest(test, "PUT", denormpath, inputBJson, 200, outputBJson, true)
	makeRequest(test, "GET", path1, "", 404, "Track Not Found\n", false)
	makeRequest(test, "GET", path2, "", 200, outputBJson, true)
	makeRequest(test, "GET", denormpath, "", 200, outputBJson, true)
}

/**
 * Checks whether a list of all tracks can be returned
 */
func TestGetAllTracks(test *testing.T) {
	clearData()
	track1url := "http://example.org/track/1256"
	escapedTrack1Url := url.QueryEscape(track1url)
	track2url := "http://example.org/track/abcdef"
	escapedTrack2Url := url.QueryEscape(track2url)
	input1Json := `{"fingerprint": "aoecu1234", "duration": 300}`
	input2Json := `{"fingerprint": "blahdebo", "duration": 150}`
	output1Json := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/1256", "trackid": 1}`
	output2Json := `{"fingerprint": "blahdebo", "duration": 150, "url": "http://example.org/track/abcdef", "trackid": 2}`
	alloutputJson1 := `[{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/1256", "trackid": 1}]`
	alloutputJson2 := `[{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/1256", "trackid": 1}, {"fingerprint": "blahdebo", "duration": 150, "url": "http://example.org/track/abcdef", "trackid": 2}]`
	path1 := fmt.Sprintf("/tracks?url=%s", escapedTrack1Url)	
	path2 := fmt.Sprintf("/tracks?url=%s", escapedTrack2Url)
	pathall := "/tracks"
	makeRequest(test, "PUT", path1, input1Json, 200, output1Json, true)
	makeRequest(test, "GET", path1, "", 200, output1Json, true)
	makeRequest(test, "GET", pathall, "", 200, alloutputJson1, true)
	makeRequest(test, "PUT", path2, input2Json, 200, output2Json, true)
	makeRequest(test, "GET", path2, "", 200, output2Json, true)
	makeRequest(test, "GET", pathall, "", 200, alloutputJson2, true)
}

/**
 * Checks whether a global value can be set and read
 */
func TestGlobals(test *testing.T) {
	clearData()
	path := "/globals/isXmas"
	inputJson := `{"value":"yes"}`
	outputJson := `{"key":"isXmas", "value":"yes"}`
	makeRequest(test, "GET", path, "", 404, "Global Variable Not Found\n", false)
	makeRequest(test, "PUT", path, inputJson, 200, outputJson, true)
	makeRequest(test, "GET", path, "", 200, outputJson, true)
	restartServer()
	makeRequest(test, "GET", path, "", 200, outputJson, true)
}
