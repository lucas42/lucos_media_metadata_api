package main

import (
	"testing"
)

func TestCreateCollection(test *testing.T) {
	clearData()
	path := "/v2/collections/basic"
	inputJson := `{"name": "A Basic Collection"}`
	outputJson := `{"slug":"basic","name": "A Basic Collection", "tracks":[]}`
	makeRequest(test, "GET", path, "", 404, "Collection Not Found\n", false)
	makeRequest(test, "PUT", path, inputJson, 200, outputJson, true)
	assertEqual(test, "Loganne event type", "collectionCreated", lastLoganneType)
	assertEqual(test, "Loganne humanReadable", "New Music Collection Created: A Basic Collection", lastLoganneMessage)
	assertEqual(test, "Loganne updatedCollection slug", "basic", lastLoganneUpdatedCollection.Slug)
	assertEqual(test, "Loganne existingCollection slug", "", lastLoganneExistingCollection.Slug)
	makeRequest(test, "GET", path, "", 200, outputJson, true)
	restartServer()
	makeRequest(test, "GET", path, "", 200, outputJson, true)
	makeRequestWithUnallowedMethod(test, path, "POST", []string{"PUT", "GET", "DELETE"})
}

/*
 * Tries to include different slugs in body, but checks that the one in the URL always takes precedent
 */
func TestCollectionWithInconsistentSlugs(test *testing.T) {
	clearData()
	path := "/v2/collections/basic"
	makeRequest(test, "GET", path, "", 404, "Collection Not Found\n", false)
	makeRequest(test, "PUT", path, `{"slug":"awkward", "name": "A Basic Collection"}`, 200, `{"slug":"basic", "name": "A Basic Collection", "tracks":[]}`, true)
	assertEqual(test, "Loganne event type", "collectionCreated", lastLoganneType)
	assertEqual(test, "Loganne humanReadable", "New Music Collection Created: A Basic Collection", lastLoganneMessage)
	assertEqual(test, "Loganne updatedCollection slug", "basic", lastLoganneUpdatedCollection.Slug)
	assertEqual(test, "Loganne existingCollection slug", "", lastLoganneExistingCollection.Slug)
	makeRequest(test, "GET", path, "", 200, `{"slug":"basic", "name": "A Basic Collection", "tracks":[]}`, true)
	restartServer() // Clear Loganne counters etc
	makeRequest(test, "PUT", path, `{"slug":"different", "name": "A Basic Collection"}`, 200, `{"slug":"basic", "name": "A Basic Collection", "tracks":[]}`, true)
	assertEqual(test, "Number of Loganne requests", 0, loganneRequestCount) // No requests to loganne made since server restart
}

func TestRenameCollection(test *testing.T) {
	clearData()
	path := "/v2/collections/basic"
	makeRequest(test, "GET", path, "", 404, "Collection Not Found\n", false)
	makeRequest(test, "PUT", path, `{"name": "A Basic Collection"}`, 200, `{"slug":"basic","name": "A Basic Collection", "tracks":[]}`, true)
	assertEqual(test, "Loganne event type", "collectionCreated", lastLoganneType)
	assertEqual(test, "Loganne humanReadable", "New Music Collection Created: A Basic Collection", lastLoganneMessage)
	assertEqual(test, "Loganne updatedCollection slug", "basic", lastLoganneUpdatedCollection.Slug)
	assertEqual(test, "Loganne existingCollection slug", "", lastLoganneExistingCollection.Slug)
	makeRequest(test, "GET", path, "", 200, `{"slug":"basic","name": "A Basic Collection", "tracks":[]}`, true)
	makeRequest(test, "PUT", path, `{"name": "Advanced Collection"}`, 200, `{"slug":"basic","name": "Advanced Collection", "tracks":[]}`, true)
	assertEqual(test, "Loganne event type", "collectionUpdated", lastLoganneType)
	assertEqual(test, "Loganne humanReadable", "Music Collection Advanced Collection Updated", lastLoganneMessage)
	assertEqual(test, "Loganne updatedCollection name", "Advanced Collection", lastLoganneUpdatedCollection.Name)
	assertEqual(test, "Loganne existingCollection name", "A Basic Collection", lastLoganneExistingCollection.Name)
	makeRequest(test, "GET", path, "", 200, `{"slug":"basic","name": "Advanced Collection", "tracks":[]}`, true)
	restartServer()
	makeRequest(test, "GET", path, "", 200, `{"slug":"basic","name": "Advanced Collection", "tracks":[]}`, true)
}

func TestRenameClashCollection(test *testing.T) {
	clearData()
	makeRequest(test, "PUT", "/v2/collections/first", `{"name": "The Collection"}`, 200, `{"slug":"first","name": "The Collection", "tracks":[]}`, true)
	makeRequest(test, "PUT", "/v2/collections/second", `{"name": "Another Collection"}`, 200, `{"slug":"second","name": "Another Collection", "tracks":[]}`, true)
	restartServer()
	makeRequest(test, "PUT", "/v2/collections/second", `{"name": "The Collection"}`, 400, "Duplicate: collection first has same name\n", false)
	assertEqual(test, "Number of Loganne requests", 0, loganneRequestCount) // No requests to loganne made since server restart
	makeRequest(test, "GET", "/v2/collections/first", "", 200, `{"slug":"first","name": "The Collection", "tracks":[]}`, true)
	makeRequest(test, "GET", "/v2/collections/second", "", 200, `{"slug":"second","name": "Another Collection", "tracks":[]}`, true)
}

/**
 * Checks whether invalid slugs are handled correctly
 */
func TestInvalidCollectionSlugs(test *testing.T) {
	clearData()
	makeRequest(test, "GET", "/v2/collections/unknown", "", 404, "Collection Not Found\n", false)
}