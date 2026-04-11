package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
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

// TestAlbumSearch checks that GET /v3/albums?q=... filters by name substring.
func TestAlbumSearch(test *testing.T) {
	clearData()
	setupRequest(test, "POST", "/v3/albums", `{"name":"Abbey Road"}`, 201)
	setupRequest(test, "POST", "/v3/albums", `{"name":"Let It Be"}`, 201)
	setupRequest(test, "POST", "/v3/albums", `{"name":"Abbey Theatre"}`, 201)

	// q=Abbey should match "Abbey Road" and "Abbey Theatre", not "Let It Be"
	request := basicRequest(test, "GET", "/v3/albums?q=Abbey", "")
	resp, _ := doRawRequest(test, request)
	var result AlbumListV3
	json.NewDecoder(resp.Body).Decode(&result)
	if result.TotalItems != 2 {
		test.Errorf("Expected 2 albums for q=Abbey, got %d", result.TotalItems)
	}
	if len(result.Albums) != 2 {
		test.Errorf("Expected 2 albums in list for q=Abbey, got %d", len(result.Albums))
	}

	// q=road (case-insensitive) should match only "Abbey Road"
	request2 := basicRequest(test, "GET", "/v3/albums?q=road", "")
	resp2, _ := doRawRequest(test, request2)
	var result2 AlbumListV3
	json.NewDecoder(resp2.Body).Decode(&result2)
	if result2.TotalItems != 1 {
		test.Errorf("Expected 1 album for q=road, got %d", result2.TotalItems)
	}

	// No q = all 3 albums
	request3 := basicRequest(test, "GET", "/v3/albums", "")
	resp3, _ := doRawRequest(test, request3)
	var result3 AlbumListV3
	json.NewDecoder(resp3.Body).Decode(&result3)
	if result3.TotalItems != 3 {
		test.Errorf("Expected 3 albums with no q, got %d", result3.TotalItems)
	}
}

// TestAlbumMethodNotAllowed checks that unsupported methods return 405.
func TestAlbumMethodNotAllowed(test *testing.T) {
	clearData()
	makeRequestWithUnallowedMethod(test, "/v3/albums", "PUT", []string{"GET", "POST"})
	makeRequestWithUnallowedMethod(test, "/v3/albums", "DELETE", []string{"GET", "POST"})
	makeRequestWithUnallowedMethod(test, "/v3/albums", "PATCH", []string{"GET", "POST"})

	setupRequest(test, "POST", "/v3/albums", `{"name":"Test Album"}`, 201)
	makeRequestWithUnallowedMethod(test, "/v3/albums/1", "POST", []string{"GET", "PUT", "DELETE"})
	makeRequestWithUnallowedMethod(test, "/v3/albums/1", "PATCH", []string{"GET", "PUT", "DELETE"})
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

// TestAlbumCreateLoganneEvent checks that creating an album fires an albumCreated Loganne event.
func TestAlbumCreateLoganneEvent(test *testing.T) {
	clearData()
	makeRequest(test, "POST", "/v3/albums", `{"name":"Revolver"}`, 201, `{"id":1,"name":"Revolver","uri":"/albums/1"}`, true)
	assertEqual(test, "loganne event type", "albumCreated", lastLoganneType)
	assertEqual(test, "loganne album name", "Revolver", lastLoganneAlbum.Name)
	assertEqual(test, "loganne album uri", "/albums/1", lastLoganneAlbum.URI)
	if !lastLoganneAlbumWithURL {
		test.Errorf("Expected albumCreated to include URL, but withURL was false")
	}
	assertEqual(test, "loganne request count", 1, loganneRequestCount)
}

// TestAlbumUpdate checks PUT /v3/albums/{id} renames the album.
func TestAlbumUpdate(test *testing.T) {
	clearData()
	setupRequest(test, "POST", "/v3/albums", `{"name":"Abbey Road"}`, 201)
	makeRequest(test, "PUT", "/v3/albums/1", `{"name":"Let It Be"}`, 200, `{"id":1,"name":"Let It Be","uri":"/albums/1"}`, true)
	// Verify the rename persisted.
	makeRequest(test, "GET", "/v3/albums/1", "", 200, `{"id":1,"name":"Let It Be","uri":"/albums/1"}`, true)
}

// TestAlbumUpdateLoganneEvent checks that updating an album fires an albumUpdated Loganne event.
func TestAlbumUpdateLoganneEvent(test *testing.T) {
	clearData()
	setupRequest(test, "POST", "/v3/albums", `{"name":"Abbey Road"}`, 201)
	makeRequest(test, "PUT", "/v3/albums/1", `{"name":"Let It Be"}`, 200, `{"id":1,"name":"Let It Be","uri":"/albums/1"}`, true)
	assertEqual(test, "loganne event type", "albumUpdated", lastLoganneType)
	assertEqual(test, "loganne album name", "Let It Be", lastLoganneAlbum.Name)
	if !lastLoganneAlbumWithURL {
		test.Errorf("Expected albumUpdated to include URL, but withURL was false")
	}
}

// TestAlbumUpdateNotFound checks PUT /v3/albums/{nonexistent} returns 404.
func TestAlbumUpdateNotFound(test *testing.T) {
	clearData()
	makeRequest(test, "PUT", "/v3/albums/999", `{"name":"Let It Be"}`, 404, `{"error":"Album Not Found","code":"not_found"}`, true)
}

// TestAlbumUpdateDuplicateName checks PUT /v3/albums/{id} returns 409 on name collision.
func TestAlbumUpdateDuplicateName(test *testing.T) {
	clearData()
	setupRequest(test, "POST", "/v3/albums", `{"name":"Abbey Road"}`, 201)
	setupRequest(test, "POST", "/v3/albums", `{"name":"Revolver"}`, 201)
	makeRequest(test, "PUT", "/v3/albums/1", `{"name":"Revolver"}`, 409, `{"error":"An album with that name already exists","code":"duplicate_name"}`, true)
}

// TestAlbumDelete checks DELETE /v3/albums/{id} removes the album.
func TestAlbumDelete(test *testing.T) {
	clearData()
	setupRequest(test, "POST", "/v3/albums", `{"name":"Abbey Road"}`, 201)
	makeRequest(test, "DELETE", "/v3/albums/1", "", 204, "", false)
	makeRequest(test, "GET", "/v3/albums/1", "", 404, `{"error":"Album Not Found","code":"not_found"}`, true)
}

// TestAlbumDeleteLoganneEvent checks that deleting an album fires an albumDeleted Loganne event without URL.
func TestAlbumDeleteLoganneEvent(test *testing.T) {
	clearData()
	setupRequest(test, "POST", "/v3/albums", `{"name":"Revolver"}`, 201)
	makeRequest(test, "DELETE", "/v3/albums/1", "", 204, "", false)
	assertEqual(test, "loganne event type", "albumDeleted", lastLoganneType)
	assertEqual(test, "loganne album name", "Revolver", lastLoganneAlbum.Name)
	if lastLoganneAlbumWithURL {
		test.Errorf("Expected albumDeleted to NOT include URL, but withURL was true")
	}
}

// TestAlbumDeleteNotFound checks DELETE /v3/albums/{nonexistent} returns 404.
func TestAlbumDeleteNotFound(test *testing.T) {
	clearData()
	makeRequest(test, "DELETE", "/v3/albums/999", "", 404, `{"error":"Album Not Found","code":"not_found"}`, true)
}

// TestAlbumDeleteInUse checks DELETE /v3/albums/{id} returns 409 when tracks reference the album.
func TestAlbumDeleteInUse(test *testing.T) {
	clearData()
	setupRequest(test, "POST", "/v3/albums", `{"name":"Abbey Road"}`, 201)
	// Create a track that references the album.
	setupRequest(test, "PUT", "/v3/tracks?url="+url.QueryEscape("http://example.org/del-in-use/1"),
		`{"fingerprint":"deltest1","duration":200,"tags":{"album":[{"uri":"/albums/1"}]}}`, 200)
	makeRequest(test, "DELETE", "/v3/albums/1", "", 409, `{"error":"Album is referenced by one or more tracks","code":"in_use"}`, true)
}

// TestAlbumRDFNotFound checks that requesting RDF for a non-existent album returns 404.
func TestAlbumRDFNotFound(test *testing.T) {
	clearData()
	request := basicRequest(test, "GET", "/v3/albums/999", "")
	request.Header.Set("Accept", "text/turtle")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		test.Error(err)
	}
	if response.StatusCode != 404 {
		test.Errorf("Expected 404 for missing album RDF, got %d", response.StatusCode)
	}
}

// TestAlbumRDFByID checks that GET /v3/albums/{id} with Accept: text/turtle returns RDF.
func TestAlbumRDFByID(test *testing.T) {
	clearData()
	os.Setenv("MEDIA_METADATA_MANAGER_ORIGIN", "http://localhost:8020")
	defer os.Unsetenv("MEDIA_METADATA_MANAGER_ORIGIN")

	setupRequest(test, "POST", "/v3/albums", `{"name":"Abbey Road"}`, 201)

	request := basicRequest(test, "GET", "/v3/albums/1", "")
	request.Header.Set("Accept", "text/turtle")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		test.Error(err)
	}
	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		test.Error(err)
	}
	body := string(responseData)

	if response.StatusCode != 200 {
		test.Errorf("Expected 200 for album RDF, got %d: %s", response.StatusCode, body)
		return
	}
	if !strings.Contains(body, "albums/1") {
		test.Errorf("Expected album URI in RDF response, got: %s", body)
	}
	if !strings.Contains(body, "mo/Record") {
		test.Errorf("Expected mo:Record type in RDF response, got: %s", body)
	}
	if !strings.Contains(body, "Abbey Road") {
		test.Errorf("Expected album name in RDF response, got: %s", body)
	}
}
