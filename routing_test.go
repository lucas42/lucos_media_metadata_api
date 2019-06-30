package main

import (
    "fmt"
    "net/http"
    "net/http/httptest"
    "net/url"
    "strings"
    "strconv"
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
	store := DBInit("testrouting.sqlite")
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
 * Constructs a http request and compares the response to some expected values
 *
 */
func makeRequest(t *testing.T, method string, path string, requestBody string, expectedResponseCode int, expectedResponseBody string, expectJSON bool) {
    reader := strings.NewReader(requestBody)
    url := server.URL + path
    request, err := http.NewRequest(method, url, reader)
    if err != nil {
        t.Error(err)
    }
    makeRawRequest(t, request, expectedResponseCode, expectedResponseBody, expectJSON)
}
/**
 * Makes a http request and compares the response to some expected values
 *
 * (Probably not the most go-eque way to do this, but it works for me)
 */
func makeRawRequest(t *testing.T, request *http.Request, expectedResponseCode int, expectedResponseBody string, expectJSON bool) {
	url := request.URL.String()
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
		if (!strings.HasPrefix(response.Header.Get("content-type"), "application/json;")) {
			t.Errorf("Expected JSON Content Type, received: \"%s\"", request.Header.Get("Content-type"))
		}
		isEqual, err := AreEqualJSON(actualResponseBody, expectedResponseBody)
		if err != nil {
			t.Errorf("Invalid JSON body: %s, expected: %s from %s", actualResponseBody, expectedResponseBody, url)
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
 * Constructs a http request with a method which shouldn't be allowed
 *
 */
func makeRequestWithUnallowedMethod(t *testing.T, path string, unallowedMethod string, allowedMethods []string) {
	url := server.URL + path
	request, err := http.NewRequest(unallowedMethod, url, nil)
	if err != nil { t.Error(err) }
	response, err := http.DefaultClient.Do(request)
	if err != nil { t.Error(err) }
	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil { t.Error(err) }
	actualResponseBody := string(responseData)
	if response.StatusCode != 405 {
		t.Errorf("Got response code %d, expected %d for %s", response.StatusCode, 405, url)
	}
	if (!strings.HasPrefix(actualResponseBody, "Method Not Allowed")) {
		t.Errorf("Repsonse body for %s doesn't begin with \"Method Not Allowed\"", url)
	}
	for _, method := range allowedMethods {
		if (!strings.Contains(response.Header.Get("allow"), method)) {
			t.Errorf("Allow header doesn't include method %s for %s", method, url)
		}
	}
}
/**
 * Constructs a http request which should do a redirect
 *
 */
func checkRedirect(t *testing.T, initialPath string, expectedPath string) {
	initialURL := server.URL + initialPath
	expectedDestination := server.URL + expectedPath
	response, err := http.Get(initialURL)
	if err != nil { t.Error(err) }
	actualDestination := response.Request.URL.String()
	if (actualDestination != expectedDestination) {
		t.Errorf("Unexpected redirect destination: \"%s\", expected: \"%s\", from \"%s\"", actualDestination, expectedDestination, initialURL)
	}
}

/**
 * Checks whether a track can be edited based on its url and retrieved later
 */
func TestCanEditTrackByUrl(test *testing.T) {
	clearData()
	trackurl := "http://example.org/track/1256"
	escapedTrackUrl := url.QueryEscape(trackurl)
	inputJson := `{"fingerprint": "aoecu1234", "duration": 300}`
	outputJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/1256", "trackid": 1, "tags": {}}`
	path := fmt.Sprintf("/tracks?url=%s", escapedTrackUrl)
	makeRequest(test, "GET", path, "", 404, "Track Not Found\n", false)
	makeRequest(test, "PUT", path, inputJson, 200, outputJson, true)
	makeRequest(test, "GET", path, "", 200, outputJson, true)
	restartServer()
	makeRequest(test, "GET", path, "", 200, outputJson, true)
	makeRequestWithUnallowedMethod(test, path, "POST", []string{"PUT", "GET"})
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
	outputAJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/333", "trackid": 1, "tags": {}}`
	outputBJson := `{"fingerprint": "aoecu1234", "duration": 150, "url": "http://example.org/track/333", "trackid": 1, "tags": {}}`
	path := fmt.Sprintf("/tracks?url=%s", escapedTrackUrl)
	makeRequest(test, "GET", path, "", 404, "Track Not Found\n", false)
	makeRequest(test, "PUT", path, inputAJson, 200, outputAJson, true)
	makeRequest(test, "PUT", path, inputBJson, 200, outputBJson, true)
	makeRequest(test, "GET", path, "", 200, outputBJson, true)
}

/**
 * Checks whether a track can be edited based on its fingerprint and retrieved later
 */
func TestCanEditTrackByFingerprint(test *testing.T) {
	clearData()
	inputJson := `{"url": "http://example.org/track/1256", "duration": 300}`
	outputJson1 := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/1256", "trackid": 1, "tags": {}}`
	outputJson2 := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/cdef", "trackid": 1, "tags": {}}`
	path := "/tracks?fingerprint=aoecu1234"
	makeRequest(test, "GET", path, "", 404, "Track Not Found\n", false)
	makeRequest(test, "PUT", path, inputJson, 200, outputJson1, true)
	makeRequest(test, "GET", path, "", 200, outputJson1, true)
	restartServer()
	makeRequest(test, "GET", path, "", 200, outputJson1, true)
	makeRequest(test, "PUT", path, `{"url": "http://example.org/track/cdef", "duration": 300}`, 200, outputJson2, true)
	makeRequest(test, "GET", path, "", 200, outputJson2, true)
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
	outputAJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/333", "trackid": 1, "tags": {}}`
	outputBJson := `{"fingerprint": "76543", "duration": 150, "url": "http://example.org/track/334", "trackid": 1, "tags": {}}`
	path1 := fmt.Sprintf("/tracks?url=%s", escapedTrack1Url)	
	path2 := fmt.Sprintf("/tracks?url=%s", escapedTrack2Url)
	path := "/tracks/1"
	makeRequest(test, "GET", path1, "", 404, "Track Not Found\n", false)
	makeRequest(test, "GET", path, "", 404, "Track Not Found\n", false)
	makeRequest(test, "PUT", path1, inputAJson, 200, outputAJson, true)
	makeRequest(test, "GET", path1, "", 200, outputAJson, true)
	makeRequest(test, "PUT", path, inputBJson, 200, outputBJson, true)
	makeRequest(test, "GET", path1, "", 404, "Track Not Found\n", false)
	makeRequest(test, "GET", path2, "", 200, outputBJson, true)
	makeRequest(test, "PUT", path, `{"start": "some JSON"`, 400, "unexpected EOF\n", false)
	makeRequest(test, "GET", path, "", 200, outputBJson, true)
}

/**
 * Checks whether non-integer track IDs are handled correctly
 */
func TestInvalidTrackIDs(test *testing.T) {
	clearData()
	makeRequest(test, "GET", "/tracks/blah", "", 404, "Track Endpoint Not Found\n", false)
	makeRequest(test, "GET", "/tracks/blah/weighting", "", 404, "Track Endpoint Not Found\n", false)
	makeRequest(test, "GET", "/tracks/1/blahing", "", 404, "Track Endpoint Not Found\n", false)
	makeRequest(test, "GET", "/tags/four/artist", "", 400, "Track ID must be an integer\n", false)
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
	output1Json := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/1256", "trackid": 1, "tags": {}}`
	output2Json := `{"fingerprint": "blahdebo", "duration": 150, "url": "http://example.org/track/abcdef", "trackid": 2, "tags": {}}`
	alloutputJson1 := `[{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/1256", "trackid": 1, "tags": {}}]`
	alloutputJson2 := `[{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/1256", "trackid": 1, "tags": {}}, {"fingerprint": "blahdebo", "duration": 150, "url": "http://example.org/track/abcdef", "trackid": 2, "tags": {}}]`
	path1 := fmt.Sprintf("/tracks?url=%s", escapedTrack1Url)	
	path2 := fmt.Sprintf("/tracks?url=%s", escapedTrack2Url)
	pathall := "/tracks"
	makeRequest(test, "PUT", path1, input1Json, 200, output1Json, true)
	makeRequest(test, "GET", path1, "", 200, output1Json, true)
	makeRequest(test, "GET", pathall, "", 200, alloutputJson1, true)
	makeRequest(test, "PUT", path2, input2Json, 200, output2Json, true)
	makeRequest(test, "GET", path2, "", 200, output2Json, true)
	makeRequest(test, "GET", pathall, "", 200, alloutputJson2, true)

	makeRequestWithUnallowedMethod(test, pathall, "PUT", []string{"GET"})
}


/**
 * Checks the pagination of the /tracks endpoint
 */
func TestPagination(test *testing.T) {
	clearData()

	// Create 45 Tracks
	for i := 1; i <= 45; i++ {
		id := strconv.Itoa(i)
		trackurl := "http://example.org/track/id"+id
		escapedTrackUrl := url.QueryEscape(trackurl)
		trackpath := fmt.Sprintf("/tracks?url=%s", escapedTrackUrl)
		inputJson := `{"fingerprint": "abcde`+id+`", "duration": 350}`
		outputJson := `{"fingerprint": "abcde`+id+`", "duration": 350, "url": "`+trackurl+`", "trackid": `+id+`, "tags": {}}`
		makeRequest(test, "PUT", trackpath, inputJson, 200, outputJson, true)
		makeRequest(test, "PUT", "/tracks/"+id+"/weighting", "4.3", 200, "4.3", false)
	}
	pages := map[string]int{
		"1": 20,
		"2": 20,
		"3": 5,
	}
	for page, count := range pages {

	    url := server.URL + "/tracks/?page=" + page
	    response, err := http.Get(url)
	    if err != nil {
	        test.Error(err)
	    }
	    responseData,err := ioutil.ReadAll(response.Body)
		if err != nil {
		    test.Error(err)
		}

		expectedResponseCode := 200
	    if response.StatusCode != expectedResponseCode {
	        test.Errorf("Got response code %d, expected %d for %s", response.StatusCode, expectedResponseCode, url)
	    }

		var output []interface{}
		err = json.Unmarshal(responseData, &output)
		if err != nil {
			test.Errorf("Invalid JSON body: %s for %s", err.Error(), "/tracks/?page=" + page)
		}
		if (len(output) != count) {
			test.Errorf("Wrong number of tracks.  Expected: %d, Actual: %d", count, len(output))
		}
	}
}

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

/**
 * Checks whether new tags can be added
 */
func TestAddTag(test *testing.T) {
	clearData()
	path1 := "/tags/1/artist"
	path2 := "/tags/1/album"
	alltagsfortrack := "/tags/1"
	allpredicates := "/predicates"
	predicate1path := "/predicates/artist"
	predicate2path := "/predicates/album"
	makeRequest(test, "GET", path1, "", 404, "Tag Not Found\n", false)
	makeRequest(test, "GET", predicate1path, "", 404, "Predicate Not Found\n", false)
	makeRequest(test, "GET", predicate2path, "", 404, "Predicate Not Found\n", false)
	makeRequest(test, "GET", allpredicates, "", 200, "[]", true)

	trackurl := "http://example.org/track/98765"
	escapedTrackUrl := url.QueryEscape(trackurl)
	trackInput := `{"fingerprint": "aoecu1234", "duration": 300}`
	trackOutput := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/98765", "trackid": 1, "tags": {}}`
	trackOutputTagged := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/98765", "trackid": 1, "tags": {"album": "un","artist":"Chumbawamba"}}`
	trackpath := fmt.Sprintf("/tracks?url=%s", escapedTrackUrl)
	makeRequest(test, "PUT", trackpath, trackInput, 200, trackOutput, true)
	makeRequest(test, "PUT", predicate1path, `{"id":"artist"}`, 200, `{"id":"artist"}`, true)
	makeRequest(test, "GET", path1, "", 404, "Tag Not Found\n", false)
	makeRequest(test, "GET", allpredicates, "", 200, `[{"id":"artist"}]`, true)

	makeRequest(test, "PUT", path1, "Chumbawamba", 200, "Chumbawamba", false)
	makeRequest(test, "GET", path1, "", 200, "Chumbawamba", false)

	makeRequest(test, "PUT", path2, "wysiwyg", 200, "wysiwyg", false)
	makeRequest(test, "GET", path2, "", 200, "wysiwyg", false)
	makeRequest(test, "GET", predicate2path, "", 200, `{"id":"album"}`, true)
	makeRequest(test, "GET", allpredicates, "", 200, `[{"id":"album"},{"id":"artist"}]`, true)

	makeRequest(test, "PUT", path2, "un", 200, "un", false)
	makeRequest(test, "GET", path2, "", 200, "un", false)
	makeRequest(test, "GET", predicate2path, "", 200, `{"id":"album"}`, true)
	makeRequest(test, "GET", allpredicates, "", 200, `[{"id":"album"},{"id":"artist"}]`, true)
	makeRequest(test, "GET", alltagsfortrack, "", 200, `{"album": "un","artist":"Chumbawamba"}`, true)
	makeRequest(test, "GET", trackpath, "", 200, trackOutputTagged, true)
	makeRequest(test, "GET", "/tracks", "", 200, "["+trackOutputTagged+"]", true)

	makeRequestWithUnallowedMethod(test, alltagsfortrack, "PUT", []string{"GET"})
	makeRequestWithUnallowedMethod(test, path1, "POST", []string{"PUT", "GET"})

	checkRedirect(test, "/tags", "/tracks")
}
/**
 * Checks whether new tags can be added
 */
func TestAddTagIfMissing(test *testing.T) {
	clearData()

	// Create track to add tag to
	trackurl := "http://example.org/track/98765"
	escapedTrackUrl := url.QueryEscape(trackurl)
	trackInput := `{"fingerprint": "aoecu1234", "duration": 300}`
	trackOutput := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/98765", "trackid": 1, "tags": {}}`
	trackOutputTagged := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/98765", "trackid": 1, "tags": {"rating":"5"}}`
	trackpath := fmt.Sprintf("/tracks?url=%s", escapedTrackUrl)
	makeRequest(test, "PUT", trackpath, trackInput, 200, trackOutput, true)

	// Add Tag (this should work as tag doesn't yet exist)
	path := "/tags/1/rating"
	reader := strings.NewReader("5")
    request, err := http.NewRequest("PUT", server.URL + path, reader)
    if err != nil {
        test.Error(err)
    }
    request.Header.Add("If-None-Match", "*")
    makeRawRequest(test, request, 200, "5", false)
	makeRequest(test, "GET", path, "", 200, "5", false)

	// Update Tag (this shouldn't have any effect as tag already exists)
	reader = strings.NewReader("2")
    request, err = http.NewRequest("PUT", server.URL + path, reader)
    if err != nil {
        test.Error(err)
    }
    request.Header.Add("If-None-Match", "*")
    makeRawRequest(test, request, 200, "5", false)
	makeRequest(test, "GET", path, "", 200, "5", false)
	makeRequest(test, "GET", trackpath, "", 200, trackOutputTagged, true)
}
/**
 * Checks whether we error correctly for tags being added to unknown tracks
 */
func TestAddTagForUnknownTrack(test *testing.T) {
	clearData()
	makeRequest(test, "PUT", "/tags/1/artist", "The Corrs", 404, "Unknown Track\n", false)
}


/**
 * Checks whether a track can have its weighting updated
 */
func TestCanUpdateWeighting(test *testing.T) {
	clearData()
	path := "/tracks/1/weighting"
	makeRequest(test, "GET", path, "", 404, "Track Not Found\n", false)

	// Create track
	trackurl := "http://example.org/track/eui4536"
	escapedTrackUrl := url.QueryEscape(trackurl)
	trackpath := fmt.Sprintf("/tracks?url=%s", escapedTrackUrl)
	outputJson := `{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/eui4536", "trackid": 1, "tags": {}}`
	makeRequest(test, "PUT", trackpath, `{"fingerprint": "aoecu1234", "duration": 300}`, 200, outputJson, true)
	
	makeRequest(test, "GET", path, "", 200, "0", false)
	makeRequest(test, "PUT", path, "bob", 400, "Weighting must be a number\n", false)
	makeRequest(test, "PUT", path, "3", 200, "3", false)
	makeRequest(test, "GET", path, "", 200, "3", false)
	restartServer()
	makeRequest(test, "GET", path, "", 200, "3", false)
	makeRequest(test, "PUT", path, "5.22", 200, "5.22", false)
	makeRequest(test, "GET", path, "", 200, "5.22", false)


	pathall := "/tracks"
	alloutputJson := `[{"fingerprint": "aoecu1234", "duration": 300, "url": "http://example.org/track/eui4536", "trackid": 1, "tags": {}, "weighting": 5.22}]`
	makeRequest(test, "GET", pathall, "", 200, alloutputJson, true)

	makeRequestWithUnallowedMethod(test, path, "POST", []string{"PUT", "GET"})
}

/**
 * Checks random tracks endpoint
 */
func TestRandomTracks(test *testing.T) {
	clearData()
	path := "/tracks/random"
	makeRequest(test, "GET", path, "", 200, "[]", true)

	// Create 40 Tracks
	for i := 1; i <= 40; i++ {
		id := strconv.Itoa(i)
		trackurl := "http://example.org/track/id"+id
		escapedTrackUrl := url.QueryEscape(trackurl)
		trackpath := fmt.Sprintf("/tracks?url=%s", escapedTrackUrl)
		inputJson := `{"fingerprint": "abcde`+id+`", "duration": 350}`
		outputJson := `{"fingerprint": "abcde`+id+`", "duration": 350, "url": "`+trackurl+`", "trackid": `+id+`, "tags": {}}`
		makeRequest(test, "PUT", trackpath, inputJson, 200, outputJson, true)
		makeRequest(test, "PUT", "/tracks/"+id+"/weighting", "4.3", 200, "4.3", false)
	}
    url := server.URL + path
    response, err := http.Get(url)
    if err != nil {
        test.Error(err)
    }
    responseData,err := ioutil.ReadAll(response.Body)
	if err != nil {
	    test.Error(err)
	}

	expectedResponseCode := 200
    if response.StatusCode != expectedResponseCode {
        test.Errorf("Got response code %d, expected %d for %s", response.StatusCode, expectedResponseCode, url)
    }

	var output []interface{}
	err = json.Unmarshal(responseData, &output)
	if err != nil {
		test.Errorf("Invalid JSON body: %s for %s", err.Error(), path)
	}
	if (len(output) != 20) {
		test.Errorf("Wrong number of tracks.  Expected: 20, Actual: %d", len(output))
	}
	if (!strings.Contains(response.Header.Get("Cache-Control"), "no-cache")) {
		test.Errorf("Random track list is cachable")
	}
	if (!strings.Contains(response.Header.Get("Cache-Control"), "max-age=0")) {
		test.Errorf("Random track missing max-age of 0")
	}
}


/**
 * Checks whether the /_info endpoint returns expected data
 */
func TestInfoEndpoint(test *testing.T) {
	clearData()

	expectedOutput := `{"system": "lucos_media_metadata_api", "checks": {"db": {"techDetail":"Does basic SELECT query from database", "ok": true}}, "metrics": {}}`
	makeRequest(test, "GET", "/_info", "", 200, expectedOutput, true)

	makeRequestWithUnallowedMethod(test, "/_info", "POST", []string{"GET"})
}