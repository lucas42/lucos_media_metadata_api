

package main

import (
    "fmt"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
    "io/ioutil"
)

var (
    server   *httptest.Server
    fingerprintUrl string
)

func init() {
    server = httptest.NewServer(routing()) //Creating new server with the user handlers

    fingerprintUrl = fmt.Sprintf("%s/fingerprint/", server.URL) //Grab the address for the API endpoint
}

func makeRequest(t *testing.T, method string, url string, requestBody string, expectedResponseCode int, expectedResponseBody string) {
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
	if (actualResponseBody != expectedResponseBody) {
		t.Errorf("Unexpected body: \"%s\", expected: \"%s\"", actualResponseBody, expectedResponseBody)
	}
}

func TestRootFingerprint(t *testing.T) {
    makeRequest(t, "GET", fingerprintUrl, "", 200, "Hello fingerprint! ")
}
func TestAddFingerprint(t *testing.T) {
    fingerprintJson := `{"fingerprint": "123456"}`
    makeRequest(t, "PUT", fingerprintUrl+"abc123", fingerprintJson, 200, "PUT not done yet abc123")
}
func TestPostFingerprint(t *testing.T) {
    fingerprintJson := `{"fingerprint": "123456"}`
    makeRequest(t, "POST", fingerprintUrl+"abc245", fingerprintJson, 405, "Error")

}