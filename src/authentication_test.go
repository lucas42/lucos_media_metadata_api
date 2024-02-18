package main

import (
    "testing"
)

func TestNoCredentials(test *testing.T) {
	request := basicRequest(test, "GET", "/v2/tracks", "")
	request.Header.Del("Authorization")
	makeRawRequest(test, request, 403, "Authentication Failed\n", false)
}
func TestUnauthorizedKey(test *testing.T) {
	request := basicRequest(test, "GET", "/v2/tracks", "")
	request.Header.Set("Authorization", "key notavalidkey")
	makeRawRequest(test, request, 403, "Authentication Failed\n", false)
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