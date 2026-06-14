package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestNoCredentials(test *testing.T) {
	request := basicRequest(test, "GET", "/v3/tracks", "")
	request.Header.Del("Authorization")
	response := makeRawRequest(test, request, 401, "Authentication Failed\n", false)
	checkResponseHeader(test, response, "WWW-Authenticate", "bearer")
}
func TestUnauthorizedKey(test *testing.T) {
	request := basicRequest(test, "GET", "/v3/tracks", "")
	request.Header.Set("Authorization", "key notavalidkey")
	response := makeRawRequest(test, request, 401, "Authentication Failed\n", false)
	checkResponseHeader(test, response, "WWW-Authenticate", "bearer")
}
func TestBearerSchemeAccepted(test *testing.T) {
	clearData()
	request := basicRequest(test, "GET", "/v3/tracks", "")
	request.Header.Set("Authorization", "bearer validkey")
	makeRawRequest(test, request, 200, `{"tracks":[],"totalPages":0,"page":1,"totalTracks":0}`, true)
}
func TestBearerSchemeV3(test *testing.T) {
	request := basicRequest(test, "GET", "/v3/tracks", "")
	request.Header.Set("Authorization", "Bearer validkey")
	makeRawRequest(test, request, 200, `{"tracks":[],"totalPages":0,"page":1,"totalTracks":0}`, true)
}
func TestBearerUnauthorized(test *testing.T) {
	request := basicRequest(test, "GET", "/v3/tracks", "")
	request.Header.Set("Authorization", "bearer notavalidkey")
	response := makeRawRequest(test, request, 401, "Authentication Failed\n", false)
	checkResponseHeader(test, response, "WWW-Authenticate", "bearer")
}
func TestBearerSchemeNoToken(test *testing.T) {
	request := basicRequest(test, "GET", "/v3/tracks", "")
	request.Header.Set("Authorization", "bearer")
	response := makeRawRequest(test, request, 401, "Authentication Failed\n", false)
	checkResponseHeader(test, response, "WWW-Authenticate", "bearer")
}
func TestUnsupportedScheme(test *testing.T) {
	request := basicRequest(test, "GET", "/v3/tracks", "")
	request.Header.Set("Authorization", "basic dXNlcjpwYXNz")
	response := makeRawRequest(test, request, 401, "Authentication Failed\n", false)
	checkResponseHeader(test, response, "WWW-Authenticate", "bearer")
}

func TestClientKeyParsing(test *testing.T) {
	singleKey := parseClientKeys("test_app:prod=abc1")
	assertEqual(test, "Wrong number of keys", 1, len(singleKey))
	assertEqual(test, "Wrong system", "test_app", singleKey["abc1"].System)
	assertEqual(test, "Wrong environment", "prod", singleKey["abc1"].Environment)

	multipleKeys := parseClientKeys("test_app:prod=abc1;other_test_app:staging=cbd234")
	assertEqual(test, "Wrong number of keys", 2, len(multipleKeys))
	assertEqual(test, "Wrong system", "test_app", multipleKeys["abc1"].System)
	assertEqual(test, "Wrong environment", "prod", multipleKeys["abc1"].Environment)
	assertEqual(test, "Wrong system", "other_test_app", multipleKeys["cbd234"].System)
	assertEqual(test, "Wrong environment", "staging", multipleKeys["cbd234"].Environment)

	keysWithSpaces := parseClientKeys(" test_app : prod= abc2;\nother_test_app : staging= cbd235   ")
	assertEqual(test, "Wrong number of keys", 2, len(keysWithSpaces))
	assertEqual(test, "Wrong system", "test_app", keysWithSpaces["abc2"].System)
	assertEqual(test, "Wrong environment", "prod", keysWithSpaces["abc2"].Environment)
	assertEqual(test, "Wrong system", "other_test_app", keysWithSpaces["cbd235"].System)
	assertEqual(test, "Wrong environment", "staging", keysWithSpaces["cbd235"].Environment)

	singleScope := parseClientKeys("test_app:prod=mykey|full")
	assertEqual(test, "Wrong number of keys", 1, len(singleScope))
	assertEqual(test, "Wrong scope count", 1, len(singleScope["mykey"].Scopes))
	assertEqual(test, "Wrong scope", "full", singleScope["mykey"].Scopes[0])

	multiScope := parseClientKeys("test_app:prod=mykey|webhook,export:read")
	assertEqual(test, "Wrong scope count", 2, len(multiScope["mykey"].Scopes))
	assertEqual(test, "Wrong first scope", "webhook", multiScope["mykey"].Scopes[0])
	assertEqual(test, "Wrong second scope", "export:read", multiScope["mykey"].Scopes[1])

	newVocabScope := parseClientKeys("test_app:prod=mykey|media-metadata:read,media-metadata:write")
	assertEqual(test, "Wrong scope count for new vocab", 2, len(newVocabScope["mykey"].Scopes))
	assertEqual(test, "Wrong first new scope", "media-metadata:read", newVocabScope["mykey"].Scopes[0])
	assertEqual(test, "Wrong second new scope", "media-metadata:write", newVocabScope["mykey"].Scopes[1])

	noScope := parseClientKeys("test_app:prod=mykey")
	assertEqual(test, "Unscoped key should have no scopes", 0, len(noScope["mykey"].Scopes))
}

// makeTestRequest creates a bare http.Request for unit-testing isAuthorized directly.
func makeTestRequest(method, path string) *http.Request {
	req, _ := http.NewRequest(method, "http://example.com"+path, nil)
	return req
}

func TestScopeEnforcement(test *testing.T) {
	webhookClient := AuthenticatedClient{System: "test", Environment: "prod", Scopes: []string{"webhook"}}
	readClient := AuthenticatedClient{System: "test", Environment: "prod", Scopes: []string{"media-metadata:read"}}
	writeClient := AuthenticatedClient{System: "test", Environment: "prod", Scopes: []string{"media-metadata:write"}}
	readWriteClient := AuthenticatedClient{System: "test", Environment: "prod", Scopes: []string{"media-metadata:read", "media-metadata:write"}}
	unscopedClient := AuthenticatedClient{System: "test", Environment: "prod", Scopes: []string{}}

	// media-metadata:read permits any GET request
	if !readClient.isAuthorized(makeTestRequest(http.MethodGet, "/v3/tracks")) {
		test.Error("media-metadata:read scope should allow GET /v3/tracks")
	}
	if !readClient.isAuthorized(makeTestRequest(http.MethodGet, "/v2/export")) {
		test.Error("media-metadata:read scope should allow GET /v2/export")
	}
	if !readClient.isAuthorized(makeTestRequest(http.MethodGet, "/v3/collections")) {
		test.Error("media-metadata:read scope should allow GET /v3/collections")
	}
	if readClient.isAuthorized(makeTestRequest(http.MethodPost, "/v3/tracks")) {
		test.Error("media-metadata:read scope should deny POST /v3/tracks")
	}
	if readClient.isAuthorized(makeTestRequest(http.MethodPatch, "/v3/tracks/1")) {
		test.Error("media-metadata:read scope should deny PATCH /v3/tracks/1")
	}
	if readClient.isAuthorized(makeTestRequest(http.MethodPost, "/webhooks")) {
		test.Error("media-metadata:read scope should deny POST /webhooks")
	}

	// media-metadata:write permits any POST/PUT/PATCH/DELETE except /webhooks
	if !writeClient.isAuthorized(makeTestRequest(http.MethodPost, "/v3/tracks")) {
		test.Error("media-metadata:write scope should allow POST /v3/tracks")
	}
	if !writeClient.isAuthorized(makeTestRequest(http.MethodPut, "/v3/tracks")) {
		test.Error("media-metadata:write scope should allow PUT /v3/tracks")
	}
	if !writeClient.isAuthorized(makeTestRequest(http.MethodPatch, "/v3/tracks/1")) {
		test.Error("media-metadata:write scope should allow PATCH /v3/tracks/1")
	}
	if !writeClient.isAuthorized(makeTestRequest(http.MethodDelete, "/v3/tracks/1")) {
		test.Error("media-metadata:write scope should allow DELETE /v3/tracks/1")
	}
	if !writeClient.isAuthorized(makeTestRequest(http.MethodPut, "/v2/tracks/1")) {
		test.Error("media-metadata:write scope should allow PUT /v2/tracks/1 (returns 410, but auth must pass first)")
	}
	if writeClient.isAuthorized(makeTestRequest(http.MethodGet, "/v3/tracks")) {
		test.Error("media-metadata:write scope should deny GET /v3/tracks")
	}
	if writeClient.isAuthorized(makeTestRequest(http.MethodPost, "/webhooks")) {
		test.Error("media-metadata:write scope should deny POST /webhooks")
	}
	if writeClient.isAuthorized(makeTestRequest(http.MethodGet, "/v2/export")) {
		test.Error("media-metadata:write scope should deny GET /v2/export")
	}

	// Combined read + write covers both
	if !readWriteClient.isAuthorized(makeTestRequest(http.MethodGet, "/v3/tracks")) {
		test.Error("read+write client should allow GET /v3/tracks")
	}
	if !readWriteClient.isAuthorized(makeTestRequest(http.MethodPost, "/v3/tracks")) {
		test.Error("read+write client should allow POST /v3/tracks")
	}
	if !readWriteClient.isAuthorized(makeTestRequest(http.MethodGet, "/v2/export")) {
		test.Error("read+write client should allow GET /v2/export")
	}
	if readWriteClient.isAuthorized(makeTestRequest(http.MethodPost, "/webhooks")) {
		test.Error("read+write client should deny POST /webhooks (needs webhook scope)")
	}

	// webhook scope only permits POST /webhooks
	if !webhookClient.isAuthorized(makeTestRequest(http.MethodPost, "/webhooks")) {
		test.Error("webhook scope should allow POST /webhooks")
	}
	if webhookClient.isAuthorized(makeTestRequest(http.MethodGet, "/v3/tracks")) {
		test.Error("webhook scope should deny GET /v3/tracks")
	}
	if webhookClient.isAuthorized(makeTestRequest(http.MethodGet, "/webhooks")) {
		test.Error("webhook scope should deny GET /webhooks")
	}
	if webhookClient.isAuthorized(makeTestRequest(http.MethodGet, "/v2/export")) {
		test.Error("webhook scope should deny GET /v2/export")
	}

	// No scope = fail-closed for all paths
	if unscopedClient.isAuthorized(makeTestRequest(http.MethodGet, "/v3/tracks")) {
		test.Error("unscoped key should be fail-closed on GET /v3/tracks")
	}
	if unscopedClient.isAuthorized(makeTestRequest(http.MethodGet, "/v2/export")) {
		test.Error("unscoped key should be fail-closed on GET /v2/export")
	}
	if unscopedClient.isAuthorized(makeTestRequest(http.MethodPost, "/webhooks")) {
		test.Error("unscoped key should be fail-closed on POST /webhooks")
	}
}

// TestUnscopedKeyReturnsForbidden verifies that a valid but unscoped key gets 403, not 401.
func TestUnscopedKeyReturnsForbidden(test *testing.T) {
	store := DBInit("testscope.sqlite", MockLoganne{})
	defer func() {
		store.DB.Close()
		testCleanup()
	}()
	testServer := httptest.NewServer(FrontController(store, "test_app:prod=noscopekey"))
	defer testServer.Close()

	req, _ := http.NewRequest(http.MethodGet, testServer.URL+"/v3/tracks", nil)
	req.Header.Set("Authorization", "bearer noscopekey")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		test.Fatal(err)
	}
	if resp.StatusCode != http.StatusForbidden {
		test.Errorf("Expected 403 Forbidden for unscoped key, got %d", resp.StatusCode)
	}
}

func testCleanup() {
	// Remove the scope-test sqlite files
	for _, f := range []string{"testscope.sqlite", "testscope.sqlite-shm", "testscope.sqlite-wal"} {
		_ = os.Remove(f)
	}
}
