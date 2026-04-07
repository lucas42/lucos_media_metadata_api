package main

import (
    "testing"
)

func TestNoCredentials(test *testing.T) {
	request := basicRequest(test, "GET", "/v3/tracks", "")
	request.Header.Del("Authorization")
	response := makeRawRequest(test, request, 401, "Authentication Failed\n", false)
	checkResponseHeader(test, response, "WWW-Authenticate", "bearer, key")
}
func TestUnauthorizedKey(test *testing.T) {
	request := basicRequest(test, "GET", "/v3/tracks", "")
	request.Header.Set("Authorization", "key notavalidkey")
	response := makeRawRequest(test, request, 401, "Authentication Failed\n", false)
	checkResponseHeader(test, response, "WWW-Authenticate", "bearer, key")
}
func TestBearerSchemeAccepted(test *testing.T) {
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
	checkResponseHeader(test, response, "WWW-Authenticate", "bearer, key")
}
func TestBearerSchemeNoToken(test *testing.T) {
	request := basicRequest(test, "GET", "/v3/tracks", "")
	request.Header.Set("Authorization", "bearer")
	response := makeRawRequest(test, request, 401, "Authentication Failed\n", false)
	checkResponseHeader(test, response, "WWW-Authenticate", "bearer, key")
}
func TestKeySchemeNoToken(test *testing.T) {
	request := basicRequest(test, "GET", "/v3/tracks", "")
	request.Header.Set("Authorization", "key")
	response := makeRawRequest(test, request, 401, "Authentication Failed\n", false)
	checkResponseHeader(test, response, "WWW-Authenticate", "bearer, key")
}
func TestUnsupportedScheme(test *testing.T) {
	request := basicRequest(test, "GET", "/v3/tracks", "")
	request.Header.Set("Authorization", "basic dXNlcjpwYXNz")
	response := makeRawRequest(test, request, 401, "Authentication Failed\n", false)
	checkResponseHeader(test, response, "WWW-Authenticate", "bearer, key")
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
}
