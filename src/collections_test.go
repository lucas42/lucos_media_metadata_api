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
func TestAddTracksToCollection(test *testing.T) {
	clearData()
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc1", `{"url":"http://example.org/track1", "duration": 7,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"}}`, 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"},"weighting": 0}`, true)
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=def2", `{"url":"http://example.org/track2", "duration": 14,"tags":{"artist":"Dropkick Murphys", "title":"The Spicy McHaggis Jig"}}`, 200, `{"fingerprint":"def2","duration":14,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"Dropkick Murphys", "title":"The Spicy McHaggis Jig"},"weighting": 0}`, true)
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=ghi3", `{"url":"http://example.org/track3", "duration": 21,"tags":{"artist":"Pachabel", "title":"Canon in D"}}`, 200, `{"fingerprint":"ghi3","duration":21,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Pachabel", "title":"Canon in D"},"weighting": 0}`, true)

	makeRequest(test, "PUT", "/v2/collections/first", `{"name": "The Collection"}`, 200, `{"slug":"first","name": "The Collection", "tracks":[]}`, true)
	makeRequest(test, "PUT", "/v2/collections/second", `{"name": "Another Collection"}`, 200, `{"slug":"second","name": "Another Collection", "tracks":[]}`, true)
	makeRequest(test, "GET", "/v2/collections/first/1", "", 404, "Track Not In Collection\n", false)
	makeRequest(test, "GET", "/v2/collections/first/2", "", 404, "Track Not In Collection\n", false)
	makeRequest(test, "GET", "/v2/collections/first/3", "", 404, "Track Not In Collection\n", false)
	makeRequest(test, "GET", "/v2/collections/first/4", "", 404, "Track Not Found\n", false) // Track 4 doesn't exist.
	makeRequest(test, "PUT", "/v2/collections/first/2", "", 200, "Track In Collection\n", false)
	makeRequest(test, "PUT", "/v2/collections/first/3", "", 200, "Track In Collection\n", false)
	makeRequest(test, "PUT", "/v2/collections/first/4", "", 404, "Track Not Found\n", false)
	makeRequest(test, "GET", "/v2/collections/first", "", 200, `{"slug":"first","name": "The Collection", "tracks":[{"fingerprint":"def2","duration":14,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"Dropkick Murphys", "title":"The Spicy McHaggis Jig"},"weighting": 0},{"fingerprint":"ghi3","duration":21,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Pachabel", "title":"Canon in D"},"weighting": 0}]}`, true)
	restartServer()
	makeRequest(test, "GET", "/v2/collections/first/1", "", 404, "Track Not In Collection\n", false)
	makeRequest(test, "GET", "/v2/collections/first/2", "", 200, "Track In Collection\n", false)
	makeRequest(test, "GET", "/v2/collections/first/3", "", 200, "Track In Collection\n", false)
	makeRequest(test, "GET", "/v2/collections/first/4", "", 404, "Track Not Found\n", false)
	makeRequest(test, "DELETE", "/v2/collections/first/3", "", 200, "Track Not In Collection\n", false) // Feels slightly odd, but 200 is correct for having deleting something
	makeRequest(test, "DELETE", "/v2/collections/first/4", "", 404, "Track Not Found\n", false)
	makeRequest(test, "GET", "/v2/collections/first", "", 200, `{"slug":"first","name": "The Collection", "tracks":[{"fingerprint":"def2","duration":14,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"Dropkick Murphys", "title":"The Spicy McHaggis Jig"},"weighting": 0}]}`, true)
	restartServer()
	makeRequest(test, "GET", "/v2/collections/first/1", "", 404, "Track Not In Collection\n", false)
	makeRequest(test, "GET", "/v2/collections/first/2", "", 200, "Track In Collection\n", false)
	makeRequest(test, "GET", "/v2/collections/first/3", "", 404, "Track Not In Collection\n", false)
	makeRequest(test, "GET", "/v2/collections/first/4", "", 404, "Track Not Found\n", false)
	makeRequest(test, "DELETE", "/v2/collections/first/3", "", 200, "Track Not In Collection\n", false) // Feels really odd as track wasn't in collection.  But DELETE should be idempotent.
	makeRequest(test, "GET", "/v2/collections/first/3", "", 404, "Track Not In Collection\n", false)
	makeRequest(test, "GET", "/v2/collections/first", "", 200, `{"slug":"first","name": "The Collection", "tracks":[{"fingerprint":"def2","duration":14,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"Dropkick Murphys", "title":"The Spicy McHaggis Jig"},"weighting": 0}]}`, true)
	makeRequest(test, "GET", "/v2/collections/second", "", 200, `{"slug":"second","name": "Another Collection", "tracks":[]}`, true)  // No requests touched this collection, so should stay empty

	makeRequest(test, "PUT", "/v2/collections/third/2", "", 404, "Collection Not Found\n", false) // Collection 'third' doesn't exist.
	makeRequest(test, "PUT", "/v2/collections/third/4", "", 404, "Collection Not Found\n", false) // Neither Collection 'third' or Track 4 exist.  Erroring on collection as it comes first in URL

	makeRequest(test, "PUT", "/v2/collections/first/something", "", 400, "Track ID must be a number\n", false)

	makeRequestWithUnallowedMethod(test, "/v2/collections/first/2", "POST", []string{"PUT", "GET", "DELETE"})
}
func TestListCollections(test *testing.T) {
	clearData()
	makeRequest(test, "GET", "/v2/collections", "", 200, `[]`, true)
	makeRequest(test, "PUT", "/v2/collections/first", `{"name": "The Collection"}`, 200, `{"slug":"first","name": "The Collection", "tracks":[]}`, true)
	makeRequest(test, "PUT", "/v2/collections/second", `{"name": "Another Collection"}`, 200, `{"slug":"second","name": "Another Collection", "tracks":[]}`, true)
	makeRequest(test, "PUT", "/v2/collections/third", `{"name": "Collecty McCollectFace"}`, 200, `{"slug":"third","name": "Collecty McCollectFace", "tracks":[]}`, true)
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc1", `{"url":"http://example.org/track1", "duration": 7,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"}}`, 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"},"weighting": 0}`, true)
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=def2", `{"url":"http://example.org/track2", "duration": 14,"tags":{"artist":"Dropkick Murphys", "title":"The Spicy McHaggis Jig"}}`, 200, `{"fingerprint":"def2","duration":14,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"Dropkick Murphys", "title":"The Spicy McHaggis Jig"},"weighting": 0}`, true)
	makeRequest(test, "PUT", "/v2/collections/first/1", "", 200, "Track In Collection\n", false)
	makeRequest(test, "PUT", "/v2/collections/first/2", "", 200, "Track In Collection\n", false)
	makeRequest(test, "PUT", "/v2/collections/third/2", "", 200, "Track In Collection\n", false)
	restartServer()
	makeRequest(test, "GET", "/v2/collections", "", 200, `[{"slug":"first","name": "The Collection", "tracks":[{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"},"weighting": 0},{"fingerprint":"def2","duration":14,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"Dropkick Murphys", "title":"The Spicy McHaggis Jig"},"weighting": 0}]},{"slug":"second","name": "Another Collection", "tracks":[]},{"slug":"third","name": "Collecty McCollectFace", "tracks":[{"fingerprint":"def2","duration":14,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"Dropkick Murphys", "title":"The Spicy McHaggis Jig"},"weighting": 0}]}]`, true)
	makeRequest(test, "GET", "/v2/collections/", "", 200, `[{"slug":"first","name": "The Collection", "tracks":[{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"},"weighting": 0},{"fingerprint":"def2","duration":14,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"Dropkick Murphys", "title":"The Spicy McHaggis Jig"},"weighting": 0}]},{"slug":"second","name": "Another Collection", "tracks":[]},{"slug":"third","name": "Collecty McCollectFace", "tracks":[{"fingerprint":"def2","duration":14,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"Dropkick Murphys", "title":"The Spicy McHaggis Jig"},"weighting": 0}]}]`, true)

}
func TestReservedCollectionSlugs(test *testing.T) {
	clearData()
	makeRequest(test, "PUT", "/v2/collections/all", `{"name": "Mega Collection"}`, 400, "Collection with slug all not allowed\n", false)
	makeRequest(test, "PUT", "/v2/collections/new", `{"name": "Shiney Collection"}`, 400, "Collection with slug new not allowed\n", false)
	makeRequest(test, "PUT", "/v2/collections/collection", `{"name": "Meta Collection"}`, 400, "Collection with slug collection not allowed\n", false)
}
func TestDeleteCollection(test *testing.T) {
	clearData()
	makeRequest(test, "PUT", "/v2/collections/first", `{"name": "The Collection"}`, 200, `{"slug":"first","name": "The Collection", "tracks":[]}`, true)
	makeRequest(test, "PUT", "/v2/collections/second", `{"name": "Another Collection"}`, 200, `{"slug":"second","name": "Another Collection", "tracks":[]}`, true)
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc1", `{"url":"http://example.org/track1", "duration": 7,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"}}`, 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"},"weighting": 0}`, true)
	makeRequest(test, "PUT", "/v2/collections/first/1", "", 200, "Track In Collection\n", false)
	makeRequest(test, "GET", "/v2/collections/first", "", 200, `{"slug":"first","name": "The Collection", "tracks":[{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"},"weighting": 0}]}`, true)
	restartServer()
	makeRequest(test, "DELETE", "/v2/collections/first", "", 204, "", false)
	assertEqual(test, "Loganne event type", "collectionDeleted", lastLoganneType)
	assertEqual(test, "Loganne humanReadable", "Collection \"The Collection\" deleted", lastLoganneMessage)
	assertEqual(test, "Loganne updatedCollection slug", "", lastLoganneUpdatedCollection.Slug)
	assertEqual(test, "Loganne existingCollection slug", "first", lastLoganneExistingCollection.Slug)
	makeRequest(test, "GET", "/v2/collections/first", "", 404, "Collection Not Found\n", false)
	makeRequest(test, "GET", "/v2/collections/first/1", "", 404, "Collection Not Found\n", false)
	makeRequest(test, "GET", "/v2/collections/second", "", 200, `{"slug":"second","name": "Another Collection", "tracks":[]}`, true)

}