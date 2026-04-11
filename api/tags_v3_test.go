package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"testing"
)

func TestV3RejectsRequiresURITagWithoutURI(t *testing.T) {
	clearData()
	trackurl := "http://example.org/v3tag/requires-uri"
	escapedTrackUrl := url.QueryEscape(trackurl)
	v3Path := fmt.Sprintf("/v3/tracks?url=%s", escapedTrackUrl)

	// language requires a URI — submitting without one should be rejected
	req := basicRequest(t, "PUT", v3Path, `{"fingerprint": "v3ruri1", "duration": 200, "tags": {"title": [{"name": "Test"}], "language": [{"name": "English"}]}}`)
	resp, _ := doRawRequest(t, req)
	if resp.StatusCode != 400 {
		t.Errorf("Expected 400 for language tag without URI, got %d", resp.StatusCode)
	}
	var errResp V3Error
	json.NewDecoder(resp.Body).Decode(&errResp)
	if errResp.Code != "requires_uri" {
		t.Errorf("Expected error code 'requires_uri', got %q", errResp.Code)
	}
}

func TestV3RejectsRequiresURITagWithEmptyURI(t *testing.T) {
	clearData()
	trackurl := "http://example.org/v3tag/requires-uri-empty"
	escapedTrackUrl := url.QueryEscape(trackurl)
	v3Path := fmt.Sprintf("/v3/tracks?url=%s", escapedTrackUrl)

	// language requires a URI — submitting with an explicit empty URI should also be rejected
	req := basicRequest(t, "PUT", v3Path, `{"fingerprint": "v3ruri2", "duration": 200, "tags": {"language": [{"name": "English", "uri": ""}]}}`)
	resp, _ := doRawRequest(t, req)
	if resp.StatusCode != 400 {
		t.Errorf("Expected 400 for language tag with empty URI, got %d", resp.StatusCode)
	}
	var errResp V3Error
	json.NewDecoder(resp.Body).Decode(&errResp)
	if errResp.Code != "requires_uri" {
		t.Errorf("Expected error code 'requires_uri', got %q", errResp.Code)
	}
}

func TestV3RejectsRequiresURIOnAboutAndMentions(t *testing.T) {
	clearData()
	trackurl := "http://example.org/v3tag/requires-uri-about"
	escapedTrackUrl := url.QueryEscape(trackurl)
	v3Path := fmt.Sprintf("/v3/tracks?url=%s", escapedTrackUrl)

	// about and mentions also require URIs
	for _, predicate := range []string{"about", "mentions"} {
		body := fmt.Sprintf(`{"fingerprint": "v3ruri-%s", "duration": 200, "tags": {"%s": [{"name": "Something"}]}}`, predicate, predicate)
		req := basicRequest(t, "PUT", v3Path, body)
		resp, _ := doRawRequest(t, req)
		if resp.StatusCode != 400 {
			t.Errorf("Expected 400 for %q tag without URI, got %d", predicate, resp.StatusCode)
		}
		var errResp V3Error
		json.NewDecoder(resp.Body).Decode(&errResp)
		if errResp.Code != "requires_uri" {
			t.Errorf("Expected error code 'requires_uri' for predicate %q, got %q", predicate, errResp.Code)
		}
	}
}

func TestV3AcceptsRequiresURITagWithValidURI(t *testing.T) {
	clearData()
	trackurl := "http://example.org/v3tag/requires-uri-valid"
	escapedTrackUrl := url.QueryEscape(trackurl)
	v3Path := fmt.Sprintf("/v3/tracks?url=%s", escapedTrackUrl)

	// language with a URI should be accepted
	req := basicRequest(t, "PUT", v3Path, `{"fingerprint": "v3ruri3", "duration": 200, "tags": {"language": [{"name": "English", "uri": "https://eolas.l42.eu/metadata/language/en/"}]}}`)
	resp, _ := doRawRequest(t, req)
	if resp.StatusCode != 200 {
		t.Errorf("Expected 200 for language tag with valid URI, got %d", resp.StatusCode)
	}
}

func TestV3RejectsRequiresURITagOnPatch(t *testing.T) {
	clearData()
	trackurl := "http://example.org/v3tag/requires-uri-patch"
	escapedTrackUrl := url.QueryEscape(trackurl)
	v3Path := fmt.Sprintf("/v3/tracks?url=%s", escapedTrackUrl)

	// Create a valid track first
	createReq := basicRequest(t, "PUT", v3Path, `{"fingerprint": "v3ruri4", "duration": 200, "tags": {"title": [{"name": "Test"}]}}`)
	resp, _ := doRawRequest(t, createReq)
	if resp.StatusCode != 200 {
		t.Fatalf("Failed to create track: %d", resp.StatusCode)
	}

	// PATCH with language without URI should be rejected
	patchReq := basicRequest(t, "PATCH", v3Path, `{"tags": {"language": [{"name": "English"}]}}`)
	resp2, _ := doRawRequest(t, patchReq)
	if resp2.StatusCode != 400 {
		t.Errorf("Expected 400 for PATCH with language tag without URI, got %d", resp2.StatusCode)
	}
	var errResp V3Error
	json.NewDecoder(resp2.Body).Decode(&errResp)
	if errResp.Code != "requires_uri" {
		t.Errorf("Expected error code 'requires_uri', got %q", errResp.Code)
	}
}

func TestTagValueV3OmitsEmptyURI(t *testing.T) {
	v := TagValueV3{Name: "Test Song"}
	data, err := json.Marshal(v)
	assertNoError(t, "MarshalJSON failed.", err)
	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	assertNoError(t, "Unmarshal raw failed.", err)
	if _, hasURI := raw["uri"]; hasURI {
		t.Error("Expected uri to be omitted when empty")
	}
	assertEqual(t, "name", "Test Song", raw["name"].(string))
}

func TestTagValueV3IncludesURI(t *testing.T) {
	v := TagValueV3{Name: "English", URI: "https://eolas.l42.eu/metadata/language/en/"}
	data, err := json.Marshal(v)
	assertNoError(t, "MarshalJSON failed.", err)
	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	assertNoError(t, "Unmarshal raw failed.", err)
	assertEqual(t, "name", "English", raw["name"].(string))
	assertEqual(t, "uri", "https://eolas.l42.eu/metadata/language/en/", raw["uri"].(string))
}
