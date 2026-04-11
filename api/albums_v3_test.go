package main

import (
	"encoding/json"
	"net/url"
	"testing"
)

// TestAlbumsGetEmpty checks that GET /v3/albums on an empty DB returns an empty list.
func TestAlbumsGetEmpty(test *testing.T) {
	clearData()
	makeRequest(test, "GET", "/v3/albums", "", 200, `{"albums":[],"totalPages":0,"page":1,"totalItems":0}`, true)
}

// TestAlbumCreate checks POST /v3/albums creates an album and returns 201.
func TestAlbumCreate(test *testing.T) {
	clearData()
	makeRequest(test, "POST", "/v3/albums", `{"name":"Abbey Road"}`, 201, `{"id":1,"name":"Abbey Road","uri":"/albums/1"}`, true)
}

// TestAlbumCreateDuplicateName checks POST /v3/albums returns 409 on duplicate name.
func TestAlbumCreateDuplicateName(test *testing.T) {
	clearData()
	makeRequest(test, "POST", "/v3/albums", `{"name":"Abbey Road"}`, 201, `{"id":1,"name":"Abbey Road","uri":"/albums/1"}`, true)
	makeRequest(test, "POST", "/v3/albums", `{"name":"Abbey Road"}`, 409, `{"error":"An album with that name already exists","code":"duplicate_name"}`, true)
}

// TestAlbumCreateNoName checks POST /v3/albums with missing/empty name returns 400.
func TestAlbumCreateNoName(test *testing.T) {
	clearData()
	makeRequest(test, "POST", "/v3/albums", `{}`, 400, `{"error":"Request body must include a non-empty \"name\" field","code":"bad_request"}`, true)
	makeRequest(test, "POST", "/v3/albums", `{"name":""}`, 400, `{"error":"Request body must include a non-empty \"name\" field","code":"bad_request"}`, true)
}

// TestAlbumGetByID checks GET /v3/albums/{id} returns the album.
func TestAlbumGetByID(test *testing.T) {
	clearData()
	setupRequest(test, "POST", "/v3/albums", `{"name":"Let It Be"}`, 201)
	makeRequest(test, "GET", "/v3/albums/1", "", 200, `{"id":1,"name":"Let It Be","uri":"/albums/1"}`, true)
}

// TestAlbumGetByIDNotFound checks GET /v3/albums/{nonexistent} returns 404.
func TestAlbumGetByIDNotFound(test *testing.T) {
	clearData()
	makeRequest(test, "GET", "/v3/albums/999", "", 404, `{"error":"Album Not Found","code":"not_found"}`, true)
}

// TestAlbumGetByIDInvalid checks GET /v3/albums/notanumber returns 404.
func TestAlbumGetByIDInvalid(test *testing.T) {
	clearData()
	makeRequest(test, "GET", "/v3/albums/notanumber", "", 404, `{"error":"Album Not Found","code":"not_found"}`, true)
}

// TestAlbumList checks GET /v3/albums returns all albums in alphabetical order.
func TestAlbumList(test *testing.T) {
	clearData()
	setupRequest(test, "POST", "/v3/albums", `{"name":"Revolver"}`, 201)
	setupRequest(test, "POST", "/v3/albums", `{"name":"Abbey Road"}`, 201)
	setupRequest(test, "POST", "/v3/albums", `{"name":"Let It Be"}`, 201)

	request := basicRequest(test, "GET", "/v3/albums", "")
	resp, _ := doRawRequest(test, request)
	if resp.StatusCode != 200 {
		test.Fatalf("Expected 200, got %d", resp.StatusCode)
	}
	var list AlbumListV3
	json.NewDecoder(resp.Body).Decode(&list)
	if list.TotalItems != 3 {
		test.Errorf("Expected 3 albums, got %d", list.TotalItems)
	}
	if len(list.Albums) != 3 {
		test.Fatalf("Expected 3 albums in list, got %d", len(list.Albums))
	}
	// Albums should be sorted alphabetically.
	assertEqual(test, "first album name", "Abbey Road", list.Albums[0].Name)
	assertEqual(test, "second album name", "Let It Be", list.Albums[1].Name)
	assertEqual(test, "third album name", "Revolver", list.Albums[2].Name)
}

// TestAlbumMethodNotAllowed checks that unsupported methods return 405.
func TestAlbumMethodNotAllowed(test *testing.T) {
	clearData()
	makeRequestWithUnallowedMethod(test, "/v3/albums", "PUT", []string{"GET", "POST"})
	makeRequestWithUnallowedMethod(test, "/v3/albums", "DELETE", []string{"GET", "POST"})
	makeRequestWithUnallowedMethod(test, "/v3/albums", "PATCH", []string{"GET", "POST"})

	setupRequest(test, "POST", "/v3/albums", `{"name":"Test Album"}`, 201)
	makeRequestWithUnallowedMethod(test, "/v3/albums/1", "DELETE", []string{"GET"})
	makeRequestWithUnallowedMethod(test, "/v3/albums/1", "PUT", []string{"GET"})
}

// TestTrackAlbumTagWrite checks that writing an album tag via URI stores the
// album name and URI, and that GET returns the tag with both name and uri.
func TestTrackAlbumTagWrite(test *testing.T) {
	clearData()
	// Create an album first.
	setupRequest(test, "POST", "/v3/albums", `{"name":"Abbey Road"}`, 201)

	// Write a track with an album tag referencing the album by URI.
	trackURL := "http://example.org/albums-test/track1"
	escapedURL := url.QueryEscape(trackURL)
	trackPath := "/v3/tracks?url=" + escapedURL
	setupRequest(test, "PUT", trackPath, `{"fingerprint":"albumtest1","duration":183,"tags":{"title":[{"name":"Come Together"}],"album":[{"uri":"/albums/1"}]}}`, 200)

	// GET the track and verify the album tag has both name and uri.
	request := basicRequest(test, "GET", trackPath, "")
	resp, _ := doRawRequest(test, request)
	var track map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&track)

	tags := track["tags"].(map[string]interface{})
	albumArr, ok := tags["album"].([]interface{})
	if !ok {
		test.Fatalf("Expected album to be an array in response, got %T", tags["album"])
	}
	if len(albumArr) != 1 {
		test.Fatalf("Expected 1 album value, got %d", len(albumArr))
	}
	albumObj := albumArr[0].(map[string]interface{})
	assertEqual(test, "album name", "Abbey Road", albumObj["name"].(string))
	assertEqual(test, "album uri", "/albums/1", albumObj["uri"].(string))
}

// TestTrackAlbumTagWriteBareNameRejected checks that writing an album tag with
// only a name (no URI) is rejected with requires_uri error.
func TestTrackAlbumTagWriteBareNameRejected(test *testing.T) {
	clearData()
	trackURL := "http://example.org/albums-test/bare-name"
	escapedURL := url.QueryEscape(trackURL)
	trackPath := "/v3/tracks?url=" + escapedURL

	req := basicRequest(test, "PUT", trackPath, `{"fingerprint":"albumtest2","duration":100,"tags":{"album":[{"name":"Abbey Road"}]}}`)
	resp, _ := doRawRequest(test, req)
	if resp.StatusCode != 400 {
		test.Errorf("Expected 400 for album tag without URI, got %d", resp.StatusCode)
	}
	var errResp V3Error
	json.NewDecoder(resp.Body).Decode(&errResp)
	if errResp.Code != "requires_uri" {
		test.Errorf("Expected error code 'requires_uri', got %q", errResp.Code)
	}
}

// TestTrackAlbumTagWriteUnknownURIRejected checks that writing an album tag
// with a URI that doesn't match an existing album is rejected.
func TestTrackAlbumTagWriteUnknownURIRejected(test *testing.T) {
	clearData()
	trackURL := "http://example.org/albums-test/unknown-uri"
	escapedURL := url.QueryEscape(trackURL)
	trackPath := "/v3/tracks?url=" + escapedURL

	req := basicRequest(test, "PUT", trackPath, `{"fingerprint":"albumtest3","duration":100,"tags":{"album":[{"uri":"/albums/999"}]}}`)
	resp, _ := doRawRequest(test, req)
	// The error wraps "Album Not Found", which writeV3Error maps to 404.
	if resp.StatusCode != 404 {
		test.Errorf("Expected 404 for album tag with unknown URI, got %d", resp.StatusCode)
	}
	var errResp V3Error
	json.NewDecoder(resp.Body).Decode(&errResp)
	if errResp.Code != "not_found" {
		test.Errorf("Expected error code 'not_found', got %q", errResp.Code)
	}
}

// TestAlbumEndpointNotFound checks /v3/albums/1/extra returns 404.
func TestAlbumEndpointNotFound(test *testing.T) {
	clearData()
	setupRequest(test, "POST", "/v3/albums", `{"name":"Test"}`, 201)
	makeRequest(test, "GET", "/v3/albums/1/extra", "", 404, `{"error":"Album Endpoint Not Found","code":"not_found"}`, true)
}

// TestAlbumPersistsAcrossRestart checks that albums survive a server restart
// (i.e. they are stored in the DB, not in-memory only).
func TestAlbumPersistsAcrossRestart(test *testing.T) {
	clearData()
	setupRequest(test, "POST", "/v3/albums", `{"name":"Persistent Album"}`, 201)
	restartServer()
	makeRequest(test, "GET", "/v3/albums/1", "", 200, `{"id":1,"name":"Persistent Album","uri":"/albums/1"}`, true)
}
