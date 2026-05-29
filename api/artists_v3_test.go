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

// TestArtistsGetEmpty checks that GET /v3/artists on an empty DB returns an empty list.
func TestArtistsGetEmpty(test *testing.T) {
	clearData()
	makeRequest(test, "GET", "/v3/artists", "", 200, `{"artists":[],"totalPages":0,"page":1,"totalItems":0}`, true)
}

// TestArtistCreate checks POST /v3/artists creates an artist and returns 201.
func TestArtistCreate(test *testing.T) {
	clearData()
	makeRequest(test, "POST", "/v3/artists", `{"name":"Enya"}`, 201, `{"id":1,"name":"Enya","uri":"/artists/1"}`, true)
}

// TestArtistCreateDuplicateName checks POST /v3/artists returns 409 on duplicate name.
func TestArtistCreateDuplicateName(test *testing.T) {
	clearData()
	makeRequest(test, "POST", "/v3/artists", `{"name":"Enya"}`, 201, `{"id":1,"name":"Enya","uri":"/artists/1"}`, true)
	makeRequest(test, "POST", "/v3/artists", `{"name":"Enya"}`, 409, `{"error":"An artist with that name already exists","code":"duplicate_name"}`, true)
}

// TestArtistCreateNoName checks POST /v3/artists with missing/empty name returns 400.
func TestArtistCreateNoName(test *testing.T) {
	clearData()
	makeRequest(test, "POST", "/v3/artists", `{}`, 400, `{"error":"Request body must include a non-empty \"name\" field","code":"bad_request"}`, true)
	makeRequest(test, "POST", "/v3/artists", `{"name":""}`, 400, `{"error":"Request body must include a non-empty \"name\" field","code":"bad_request"}`, true)
}

// TestArtistGetByID checks GET /v3/artists/{id} returns the artist.
func TestArtistGetByID(test *testing.T) {
	clearData()
	setupRequest(test, "POST", "/v3/artists", `{"name":"Clannad"}`, 201)
	makeRequest(test, "GET", "/v3/artists/1", "", 200, `{"id":1,"name":"Clannad","uri":"/artists/1"}`, true)
}

// TestArtistGetByIDNotFound checks GET /v3/artists/{nonexistent} returns 404.
func TestArtistGetByIDNotFound(test *testing.T) {
	clearData()
	makeRequest(test, "GET", "/v3/artists/999", "", 404, `{"error":"Artist Not Found","code":"not_found"}`, true)
}

// TestArtistGetByIDInvalid checks GET /v3/artists/notanumber returns 404.
func TestArtistGetByIDInvalid(test *testing.T) {
	clearData()
	makeRequest(test, "GET", "/v3/artists/notanumber", "", 404, `{"error":"Artist Not Found","code":"not_found"}`, true)
}

// TestArtistList checks GET /v3/artists returns all artists in alphabetical order.
func TestArtistList(test *testing.T) {
	clearData()
	setupRequest(test, "POST", "/v3/artists", `{"name":"Enya"}`, 201)
	setupRequest(test, "POST", "/v3/artists", `{"name":"Clannad"}`, 201)
	setupRequest(test, "POST", "/v3/artists", `{"name":"Fatboy Slim"}`, 201)

	request := basicRequest(test, "GET", "/v3/artists", "")
	resp, _ := doRawRequest(test, request)
	if resp.StatusCode != 200 {
		test.Fatalf("Expected 200, got %d", resp.StatusCode)
	}
	var list ArtistListV3
	json.NewDecoder(resp.Body).Decode(&list)
	if list.TotalItems != 3 {
		test.Errorf("Expected 3 artists, got %d", list.TotalItems)
	}
	if len(list.Artists) != 3 {
		test.Fatalf("Expected 3 artists in list, got %d", len(list.Artists))
	}
	// Alphabetical order
	assertEqual(test, "first artist name", "Clannad", list.Artists[0].Name)
	assertEqual(test, "second artist name", "Enya", list.Artists[1].Name)
	assertEqual(test, "third artist name", "Fatboy Slim", list.Artists[2].Name)
}

// TestArtistSearch checks GET /v3/artists?q=... filters by name substring.
func TestArtistSearch(test *testing.T) {
	clearData()
	setupRequest(test, "POST", "/v3/artists", `{"name":"Enya"}`, 201)
	setupRequest(test, "POST", "/v3/artists", `{"name":"Clannad"}`, 201)
	setupRequest(test, "POST", "/v3/artists", `{"name":"Eithne Brennan"}`, 201)

	// q=Ei should match "Eithne Brennan" but not "Enya" or "Clannad"
	request := basicRequest(test, "GET", "/v3/artists?q=Eithne", "")
	resp, _ := doRawRequest(test, request)
	var result ArtistListV3
	json.NewDecoder(resp.Body).Decode(&result)
	if result.TotalItems != 1 {
		test.Errorf("Expected 1 artist for q=Eithne, got %d", result.TotalItems)
	}

	// No q = all 3 artists
	request2 := basicRequest(test, "GET", "/v3/artists", "")
	resp2, _ := doRawRequest(test, request2)
	var result2 ArtistListV3
	json.NewDecoder(resp2.Body).Decode(&result2)
	if result2.TotalItems != 3 {
		test.Errorf("Expected 3 artists with no q, got %d", result2.TotalItems)
	}
}

// TestArtistMethodNotAllowed checks that unsupported methods return 405.
func TestArtistMethodNotAllowed(test *testing.T) {
	clearData()
	makeRequestWithUnallowedMethod(test, "/v3/artists", "PUT", []string{"GET", "POST"})
	makeRequestWithUnallowedMethod(test, "/v3/artists", "DELETE", []string{"GET", "POST"})

	setupRequest(test, "POST", "/v3/artists", `{"name":"Enya"}`, 201)
	makeRequestWithUnallowedMethod(test, "/v3/artists/1", "POST", []string{"GET", "PUT", "DELETE"})
}

// TestArtistUpdate checks PUT /v3/artists/{id} renames the artist.
func TestArtistUpdate(test *testing.T) {
	clearData()
	setupRequest(test, "POST", "/v3/artists", `{"name":"Enya"}`, 201)
	makeRequest(test, "PUT", "/v3/artists/1", `{"name":"Clannad"}`, 200, `{"id":1,"name":"Clannad","uri":"/artists/1"}`, true)
	makeRequest(test, "GET", "/v3/artists/1", "", 200, `{"id":1,"name":"Clannad","uri":"/artists/1"}`, true)
}

// TestArtistUpdateCascadesToTags checks that renaming an artist cascades the name
// change to all tag rows that reference the artist.
func TestArtistUpdateCascadesToTags(test *testing.T) {
	clearData()
	setupRequest(test, "POST", "/v3/artists", `{"name":"Enya"}`, 201)
	trackURL := "http://example.org/artist-cascade-test/track1"
	escapedURL := url.QueryEscape(trackURL)
	trackPath := "/v3/tracks?url=" + escapedURL
	setupRequest(test, "PUT", trackPath, `{"fingerprint":"artistcascade1","duration":183,"tags":{"artist":[{"uri":"/artists/1"}]}}`, 200)

	// Rename the artist
	setupRequest(test, "PUT", "/v3/artists/1", `{"name":"Clannad"}`, 200)

	// Verify the tag's value was cascaded
	getReq := basicRequest(test, "GET", trackPath, "")
	getResp, _ := doRawRequest(test, getReq)
	var track map[string]interface{}
	json.NewDecoder(getResp.Body).Decode(&track)
	tags := track["tags"].(map[string]interface{})
	artistArr := tags["artist"].([]interface{})
	artistObj := artistArr[0].(map[string]interface{})
	assertEqual(test, "artist name after rename", "Clannad", artistObj["name"].(string))
	assertEqual(test, "artist uri unchanged", "/artists/1", artistObj["uri"].(string))
}

// TestArtistUpdateNotFound checks PUT /v3/artists/{nonexistent} returns 404.
func TestArtistUpdateNotFound(test *testing.T) {
	clearData()
	makeRequest(test, "PUT", "/v3/artists/999", `{"name":"Enya"}`, 404, `{"error":"Artist Not Found","code":"not_found"}`, true)
}

// TestArtistUpdateDuplicateName checks PUT /v3/artists/{id} returns 409 on name collision.
func TestArtistUpdateDuplicateName(test *testing.T) {
	clearData()
	setupRequest(test, "POST", "/v3/artists", `{"name":"Enya"}`, 201)
	setupRequest(test, "POST", "/v3/artists", `{"name":"Clannad"}`, 201)
	makeRequest(test, "PUT", "/v3/artists/1", `{"name":"Clannad"}`, 409, `{"error":"An artist with that name already exists","code":"duplicate_name"}`, true)
}

// TestArtistDelete checks DELETE /v3/artists/{id} removes the artist.
func TestArtistDelete(test *testing.T) {
	clearData()
	setupRequest(test, "POST", "/v3/artists", `{"name":"Enya"}`, 201)
	makeRequest(test, "DELETE", "/v3/artists/1", "", 204, "", false)
	makeRequest(test, "GET", "/v3/artists/1", "", 404, `{"error":"Artist Not Found","code":"not_found"}`, true)
}

// TestArtistDeleteNotFound checks DELETE /v3/artists/{nonexistent} returns 404.
func TestArtistDeleteNotFound(test *testing.T) {
	clearData()
	makeRequest(test, "DELETE", "/v3/artists/999", "", 404, `{"error":"Artist Not Found","code":"not_found"}`, true)
}

// TestArtistDeleteInUse checks DELETE /v3/artists/{id} returns 409 when tracks reference it.
func TestArtistDeleteInUse(test *testing.T) {
	clearData()
	setupRequest(test, "POST", "/v3/artists", `{"name":"Enya"}`, 201)
	setupRequest(test, "PUT", "/v3/tracks?url="+url.QueryEscape("http://example.org/artist-del-in-use/1"),
		`{"fingerprint":"artistdeltest1","duration":200,"tags":{"artist":[{"uri":"/artists/1"}]}}`, 200)
	makeRequest(test, "DELETE", "/v3/artists/1", "", 409, `{"error":"Artist is referenced by one or more tracks","code":"in_use"}`, true)
}

// TestArtistPersistsAcrossRestart checks that artists survive a server restart.
func TestArtistPersistsAcrossRestart(test *testing.T) {
	clearData()
	setupRequest(test, "POST", "/v3/artists", `{"name":"Persistent Artist"}`, 201)
	restartServer()
	makeRequest(test, "GET", "/v3/artists/1", "", 200, `{"id":1,"name":"Persistent Artist","uri":"/artists/1"}`, true)
}

// TestArtistCreateLoganneEvent checks that creating an artist fires an artistCreated Loganne event.
func TestArtistCreateLoganneEvent(test *testing.T) {
	clearData()
	makeRequest(test, "POST", "/v3/artists", `{"name":"Enya"}`, 201, `{"id":1,"name":"Enya","uri":"/artists/1"}`, true)
	assertEqual(test, "loganne event type", "artistCreated", lastLoganneType)
	assertEqual(test, "loganne artist name", "Enya", lastLoganneArtist.Name)
	assertEqual(test, "loganne artist uri", "/artists/1", lastLoganneArtist.URI)
	if !lastLoganneArtistWithURL {
		test.Errorf("Expected artistCreated to include URL, but withURL was false")
	}
	assertEqual(test, "loganne request count", 1, loganneRequestCount)
}

// TestArtistDeleteLoganneEvent checks that deleting an artist fires an artistDeleted event without URL.
func TestArtistDeleteLoganneEvent(test *testing.T) {
	clearData()
	setupRequest(test, "POST", "/v3/artists", `{"name":"Enya"}`, 201)
	makeRequest(test, "DELETE", "/v3/artists/1", "", 204, "", false)
	assertEqual(test, "loganne event type", "artistDeleted", lastLoganneType)
	assertEqual(test, "loganne artist name", "Enya", lastLoganneArtist.Name)
	if lastLoganneArtistWithURL {
		test.Errorf("Expected artistDeleted to NOT include URL, but withURL was true")
	}
}

// TestArtistUpdateLoganneEvent checks that updating an artist fires an artistUpdated Loganne event.
func TestArtistUpdateLoganneEvent(test *testing.T) {
	clearData()
	setupRequest(test, "POST", "/v3/artists", `{"name":"Enya"}`, 201)
	makeRequest(test, "PUT", "/v3/artists/1", `{"name":"Clannad"}`, 200, `{"id":1,"name":"Clannad","uri":"/artists/1"}`, true)
	assertEqual(test, "loganne event type", "artistUpdated", lastLoganneType)
	assertEqual(test, "loganne artist name", "Clannad", lastLoganneArtist.Name)
	if !lastLoganneArtistWithURL {
		test.Errorf("Expected artistUpdated to include URL, but withURL was false")
	}
}

// TestArtistRDFNotFound checks that requesting RDF for a non-existent artist returns 404.
func TestArtistRDFNotFound(test *testing.T) {
	clearData()
	request := basicRequest(test, "GET", "/v3/artists/999", "")
	request.Header.Set("Accept", "text/turtle")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		test.Error(err)
	}
	if response.StatusCode != 404 {
		test.Errorf("Expected 404 for missing artist RDF, got %d", response.StatusCode)
	}
}

// TestArtistRDFByID checks that GET /v3/artists/{id} with Accept: text/turtle returns RDF.
func TestArtistRDFByID(test *testing.T) {
	clearData()
	os.Setenv("MEDIA_METADATA_MANAGER_ORIGIN", "http://localhost:8020")
	defer os.Unsetenv("MEDIA_METADATA_MANAGER_ORIGIN")

	setupRequest(test, "POST", "/v3/artists", `{"name":"Enya"}`, 201)

	request := basicRequest(test, "GET", "/v3/artists/1", "")
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
		test.Errorf("Expected 200 for artist RDF, got %d: %s", response.StatusCode, body)
		return
	}
	if !strings.Contains(body, "artists/1") {
		test.Errorf("Expected artist URI in RDF response, got: %s", body)
	}
	if !strings.Contains(body, "mo/MusicArtist") {
		test.Errorf("Expected mo:MusicArtist type in RDF response, got: %s", body)
	}
	if !strings.Contains(body, "Enya") {
		test.Errorf("Expected artist name in RDF response, got: %s", body)
	}
}

// TestArtistEndpointNotFound checks /v3/artists/1/extra returns 404.
func TestArtistEndpointNotFound(test *testing.T) {
	clearData()
	setupRequest(test, "POST", "/v3/artists", `{"name":"Enya"}`, 201)
	makeRequest(test, "GET", "/v3/artists/1/extra", "", 404, `{"error":"Artist Endpoint Not Found","code":"not_found"}`, true)
}

// TestArtistMerge checks POST /v3/artists/merge merges a source into target.
func TestArtistMerge(test *testing.T) {
	clearData()
	setupRequest(test, "POST", "/v3/artists", `{"name":"Enya"}`, 201)   // id=1
	setupRequest(test, "POST", "/v3/artists", `{"name":"Clannad"}`, 201) // id=2

	makeRequest(test, "POST", "/v3/artists/merge", `{"targetId":1,"sourceIds":[2]}`, 200, `{"id":1,"name":"Enya","uri":"/artists/1"}`, true)
	makeRequest(test, "GET", "/v3/artists/2", "", 404, `{"error":"Artist Not Found","code":"not_found"}`, true)
	makeRequest(test, "GET", "/v3/artists/1", "", 200, `{"id":1,"name":"Enya","uri":"/artists/1"}`, true)
}

// TestArtistMergeRepoints checks that track artist tags referencing source artists are repointed.
func TestArtistMergeRepoints(test *testing.T) {
	clearData()
	setupRequest(test, "POST", "/v3/artists", `{"name":"Enya"}`, 201)   // id=1 target
	setupRequest(test, "POST", "/v3/artists", `{"name":"Clannad"}`, 201) // id=2 source

	trackPath := "/v3/tracks?url=" + url.QueryEscape("http://example.org/artist-merge-test/track1")
	setupRequest(test, "PUT", trackPath, `{"fingerprint":"artistmerge1","duration":200,"tags":{"artist":[{"uri":"/artists/2"}]}}`, 200)

	setupRequest(test, "POST", "/v3/artists/merge", `{"targetId":1,"sourceIds":[2]}`, 200)

	request := basicRequest(test, "GET", trackPath, "")
	resp, _ := doRawRequest(test, request)
	var track map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&track)
	tags := track["tags"].(map[string]interface{})
	artistArr := tags["artist"].([]interface{})
	artistObj := artistArr[0].(map[string]interface{})
	assertEqual(test, "artist uri after merge", "/artists/1", artistObj["uri"].(string))
	assertEqual(test, "artist name after merge", "Enya", artistObj["name"].(string))
}

// TestArtistMergeLoganneEvent checks that merging emits an artistMerged Loganne event per source.
func TestArtistMergeLoganneEvent(test *testing.T) {
	clearData()
	setupRequest(test, "POST", "/v3/artists", `{"name":"Enya"}`, 201)   // id=1 target
	setupRequest(test, "POST", "/v3/artists", `{"name":"Clannad"}`, 201) // id=2 source
	loganneRequestCount = 0

	makeRequest(test, "POST", "/v3/artists/merge", `{"targetId":1,"sourceIds":[2]}`, 200, `{"id":1,"name":"Enya","uri":"/artists/1"}`, true)

	assertEqual(test, "loganne event type", "artistMerged", lastLoganneType)
	assertEqual(test, "loganne source artist name", "Clannad", lastLoganneArtist.Name)
	assertEqual(test, "loganne source artist uri", "/artists/2", lastLoganneArtist.URI)
	assertEqual(test, "loganne target artist name", "Enya", lastLoganneTargetArtist.Name)
	assertEqual(test, "loganne target artist uri", "/artists/1", lastLoganneTargetArtist.URI)
	assertEqual(test, "loganne request count", 1, loganneRequestCount)
}

// TestArtistMergeBadRequest checks merge validation errors.
func TestArtistMergeBadRequest(test *testing.T) {
	clearData()
	makeRequest(test, "POST", "/v3/artists/merge", `{"sourceIds":[1]}`, 400, `{"error":"Request body must include a positive \"targetId\" and a non-empty \"sourceIds\" array","code":"bad_request"}`, true)
	makeRequest(test, "POST", "/v3/artists/merge", `{"targetId":1}`, 400, `{"error":"Request body must include a positive \"targetId\" and a non-empty \"sourceIds\" array","code":"bad_request"}`, true)
	setupRequest(test, "POST", "/v3/artists", `{"name":"Enya"}`, 201)
	makeRequest(test, "POST", "/v3/artists/merge", `{"targetId":1,"sourceIds":[1]}`, 400, `{"error":"Target artist cannot also be listed as a source","code":"bad_request"}`, true)
}

// TestArtistMergeMethodNotAllowed checks that non-POST methods on /v3/artists/merge return 405.
func TestArtistMergeMethodNotAllowed(test *testing.T) {
	clearData()
	makeRequestWithUnallowedMethod(test, "/v3/artists/merge", "GET", []string{"POST"})
	makeRequestWithUnallowedMethod(test, "/v3/artists/merge", "PUT", []string{"POST"})
	makeRequestWithUnallowedMethod(test, "/v3/artists/merge", "DELETE", []string{"POST"})
}

// TestTrackArtistTagWrite checks that writing an artist tag via URI stores the
// artist name and URI, and that GET returns the tag with both name and uri.
func TestTrackArtistTagWrite(test *testing.T) {
	clearData()
	setupRequest(test, "POST", "/v3/artists", `{"name":"Enya"}`, 201)

	trackURL := "http://example.org/artist-test/track1"
	escapedURL := url.QueryEscape(trackURL)
	trackPath := "/v3/tracks?url=" + escapedURL
	setupRequest(test, "PUT", trackPath, `{"fingerprint":"artisttest1","duration":183,"tags":{"title":[{"name":"Orinoco Flow"}],"artist":[{"uri":"/artists/1"}]}}`, 200)

	request := basicRequest(test, "GET", trackPath, "")
	resp, _ := doRawRequest(test, request)
	var track map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&track)

	tags := track["tags"].(map[string]interface{})
	artistArr, ok := tags["artist"].([]interface{})
	if !ok {
		test.Fatalf("Expected artist to be an array in response, got %T", tags["artist"])
	}
	if len(artistArr) != 1 {
		test.Fatalf("Expected 1 artist value, got %d", len(artistArr))
	}
	artistObj := artistArr[0].(map[string]interface{})
	assertEqual(test, "artist name", "Enya", artistObj["name"].(string))
	assertEqual(test, "artist uri", "/artists/1", artistObj["uri"].(string))
}

// TestTrackArtistTagWriteByNameCreatesArtist checks that writing an artist tag with
// only a name creates the artist and stores the tag correctly.
func TestTrackArtistTagWriteByNameCreatesArtist(test *testing.T) {
	clearData()
	trackURL := "http://example.org/artist-test/bare-name"
	escapedURL := url.QueryEscape(trackURL)
	trackPath := "/v3/tracks?url=" + escapedURL

	req := basicRequest(test, "PUT", trackPath, `{"fingerprint":"artisttest2","duration":100,"tags":{"artist":[{"name":"Enya"}]}}`)
	resp, _ := doRawRequest(test, req)
	if resp.StatusCode != 200 {
		test.Errorf("Expected 200 for artist tag with bare name, got %d", resp.StatusCode)
	}

	// The artist should have been created.
	makeRequest(test, "GET", "/v3/artists/1", "", 200, `{"id":1,"name":"Enya","uri":"/artists/1"}`, true)

	// The track should have the artist tag with both name and URI.
	getReq := basicRequest(test, "GET", trackPath, "")
	getResp, _ := doRawRequest(test, getReq)
	var track map[string]interface{}
	json.NewDecoder(getResp.Body).Decode(&track)
	tags := track["tags"].(map[string]interface{})
	artistArr, ok := tags["artist"].([]interface{})
	if !ok || len(artistArr) != 1 {
		test.Fatalf("Expected 1 artist tag in response")
	}
	artistObj := artistArr[0].(map[string]interface{})
	assertEqual(test, "artist name", "Enya", artistObj["name"].(string))
	if artistObj["uri"] == nil || artistObj["uri"].(string) == "" {
		test.Error("Expected artist URI to be populated after bare-name write")
	}
}

// TestTrackArtistTagWriteByNameResolvesExisting checks that writing an artist tag
// with only a name resolves to an existing artist rather than creating a duplicate.
func TestTrackArtistTagWriteByNameResolvesExisting(test *testing.T) {
	clearData()
	setupRequest(test, "POST", "/v3/artists", `{"name":"Enya"}`, 201)

	trackURL := "http://example.org/artist-test/bare-name-existing"
	escapedURL := url.QueryEscape(trackURL)
	trackPath := "/v3/tracks?url=" + escapedURL

	req := basicRequest(test, "PUT", trackPath, `{"fingerprint":"artisttest2b","duration":100,"tags":{"artist":[{"name":"Enya"}]}}`)
	resp, _ := doRawRequest(test, req)
	if resp.StatusCode != 200 {
		test.Errorf("Expected 200 for artist tag with bare name matching existing artist, got %d", resp.StatusCode)
	}

	// Only one artist should exist (no duplicate created).
	listReq := basicRequest(test, "GET", "/v3/artists", "")
	listResp, _ := doRawRequest(test, listReq)
	var list ArtistListV3
	json.NewDecoder(listResp.Body).Decode(&list)
	if list.TotalItems != 1 {
		test.Errorf("Expected 1 artist (no duplicate created), got %d", list.TotalItems)
	}

	// The track's artist tag should reference artist id=1.
	getReq := basicRequest(test, "GET", trackPath, "")
	getResp, _ := doRawRequest(test, getReq)
	var track map[string]interface{}
	json.NewDecoder(getResp.Body).Decode(&track)
	tags := track["tags"].(map[string]interface{})
	artistArr := tags["artist"].([]interface{})
	artistObj := artistArr[0].(map[string]interface{})
	assertEqual(test, "artist uri", "/artists/1", artistObj["uri"].(string))
}

// TestTrackMultipleArtists checks that a track can have multiple artist tags.
func TestTrackMultipleArtists(test *testing.T) {
	clearData()
	setupRequest(test, "POST", "/v3/artists", `{"name":"Enya"}`, 201)   // id=1
	setupRequest(test, "POST", "/v3/artists", `{"name":"Clannad"}`, 201) // id=2

	trackURL := "http://example.org/artist-test/multi"
	escapedURL := url.QueryEscape(trackURL)
	trackPath := "/v3/tracks?url=" + escapedURL

	req := basicRequest(test, "PUT", trackPath, `{"fingerprint":"artistmulti1","duration":240,"tags":{"artist":[{"uri":"/artists/1"},{"uri":"/artists/2"}]}}`)
	resp, _ := doRawRequest(test, req)
	if resp.StatusCode != 200 {
		test.Errorf("Expected 200 for multi-artist write, got %d", resp.StatusCode)
	}

	getReq := basicRequest(test, "GET", trackPath, "")
	getResp, _ := doRawRequest(test, getReq)
	var track map[string]interface{}
	json.NewDecoder(getResp.Body).Decode(&track)
	tags := track["tags"].(map[string]interface{})
	artistArr, ok := tags["artist"].([]interface{})
	if !ok {
		test.Fatalf("Expected artist to be an array in response, got %T", tags["artist"])
	}
	if len(artistArr) != 2 {
		test.Errorf("Expected 2 artist values, got %d", len(artistArr))
	}
}
