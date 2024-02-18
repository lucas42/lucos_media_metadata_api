package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"io/ioutil"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestCreateCollection(test *testing.T) {
	clearData()
	path := "/v2/collections/basic"
	inputJson := `{"name": "A Basic Collection"}`
	outputJson := `{"slug":"basic","name": "A Basic Collection", "tracks":[], "totalPages":0}`
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
	makeRequest(test, "PUT", path, `{"slug":"awkward", "name": "A Basic Collection"}`, 200, `{"slug":"basic", "name": "A Basic Collection", "tracks":[], "totalPages":0}`, true)
	assertEqual(test, "Loganne event type", "collectionCreated", lastLoganneType)
	assertEqual(test, "Loganne humanReadable", "New Music Collection Created: A Basic Collection", lastLoganneMessage)
	assertEqual(test, "Loganne updatedCollection slug", "basic", lastLoganneUpdatedCollection.Slug)
	assertEqual(test, "Loganne existingCollection slug", "", lastLoganneExistingCollection.Slug)
	makeRequest(test, "GET", path, "", 200, `{"slug":"basic", "name": "A Basic Collection", "tracks":[], "totalPages":0}`, true)
	restartServer() // Clear Loganne counters etc
	makeRequest(test, "PUT", path, `{"slug":"different", "name": "A Basic Collection"}`, 200, `{"slug":"basic", "name": "A Basic Collection", "tracks":[], "totalPages":0}`, true)
	assertEqual(test, "Number of Loganne requests", 0, loganneRequestCount) // No requests to loganne made since server restart
}

func TestRenameCollection(test *testing.T) {
	clearData()
	path := "/v2/collections/basic"
	makeRequest(test, "GET", path, "", 404, "Collection Not Found\n", false)
	makeRequest(test, "PUT", path, `{"name": "A Basic Collection"}`, 200, `{"slug":"basic","name": "A Basic Collection", "tracks":[], "totalPages":0}`, true)
	assertEqual(test, "Loganne event type", "collectionCreated", lastLoganneType)
	assertEqual(test, "Loganne humanReadable", "New Music Collection Created: A Basic Collection", lastLoganneMessage)
	assertEqual(test, "Loganne updatedCollection slug", "basic", lastLoganneUpdatedCollection.Slug)
	assertEqual(test, "Loganne existingCollection slug", "", lastLoganneExistingCollection.Slug)
	makeRequest(test, "GET", path, "", 200, `{"slug":"basic","name": "A Basic Collection", "tracks":[], "totalPages":0}`, true)
	makeRequest(test, "PUT", path, `{"name": "Advanced Collection"}`, 200, `{"slug":"basic","name": "Advanced Collection", "tracks":[], "totalPages":0}`, true)
	assertEqual(test, "Loganne event type", "collectionUpdated", lastLoganneType)
	assertEqual(test, "Loganne humanReadable", "Music Collection Advanced Collection Updated", lastLoganneMessage)
	assertEqual(test, "Loganne updatedCollection name", "Advanced Collection", lastLoganneUpdatedCollection.Name)
	assertEqual(test, "Loganne existingCollection name", "A Basic Collection", lastLoganneExistingCollection.Name)
	makeRequest(test, "GET", path, "", 200, `{"slug":"basic","name": "Advanced Collection", "tracks":[], "totalPages":0}`, true)
	restartServer()
	makeRequest(test, "GET", path, "", 200, `{"slug":"basic","name": "Advanced Collection", "tracks":[], "totalPages":0}`, true)
}

func TestRenameClashCollection(test *testing.T) {
	clearData()
	makeRequest(test, "PUT", "/v2/collections/first", `{"name": "The Collection"}`, 200, `{"slug":"first","name": "The Collection", "tracks":[], "totalPages":0}`, true)
	makeRequest(test, "PUT", "/v2/collections/second", `{"name": "Another Collection"}`, 200, `{"slug":"second","name": "Another Collection", "tracks":[], "totalPages":0}`, true)
	restartServer()
	makeRequest(test, "PUT", "/v2/collections/second", `{"name": "The Collection"}`, 400, "Duplicate: collection first has same name\n", false)
	assertEqual(test, "Number of Loganne requests", 0, loganneRequestCount) // No requests to loganne made since server restart
	makeRequest(test, "GET", "/v2/collections/first", "", 200, `{"slug":"first","name": "The Collection", "tracks":[], "totalPages":0}`, true)
	makeRequest(test, "GET", "/v2/collections/second", "", 200, `{"slug":"second","name": "Another Collection", "tracks":[], "totalPages":0}`, true)
}
func TestAddTracksToCollectionOnDedicatedEndpoint(test *testing.T) {
	clearData()
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc1", `{"url":"http://example.org/track1", "duration": 7,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"}}`, 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"},"collections":[],"weighting": 0}`, true)
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=def2", `{"url":"http://example.org/track2", "duration": 14,"tags":{"artist":"Dropkick Murphys", "title":"The Spicy McHaggis Jig"}}`, 200, `{"fingerprint":"def2","duration":14,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"Dropkick Murphys", "title":"The Spicy McHaggis Jig"},"collections":[],"weighting": 0}`, true)
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=ghi3", `{"url":"http://example.org/track3", "duration": 21,"tags":{"artist":"Pachabel", "title":"Canon in D"}}`, 200, `{"fingerprint":"ghi3","duration":21,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Pachabel", "title":"Canon in D"},"collections":[],"weighting": 0}`, true)

	makeRequest(test, "PUT", "/v2/collections/first", `{"name": "The Collection"}`, 200, `{"slug":"first","name": "The Collection", "tracks":[], "totalPages":0}`, true)
	makeRequest(test, "PUT", "/v2/collections/second", `{"name": "Another Collection"}`, 200, `{"slug":"second","name": "Another Collection", "tracks":[], "totalPages":0}`, true)
	makeRequest(test, "GET", "/v2/collections/first/1", "", 404, "Track Not In Collection\n", false)
	makeRequest(test, "GET", "/v2/collections/first/2", "", 404, "Track Not In Collection\n", false)
	makeRequest(test, "GET", "/v2/collections/first/3", "", 404, "Track Not In Collection\n", false)
	makeRequest(test, "GET", "/v2/collections/first/4", "", 404, "Track Not Found\n", false) // Track 4 doesn't exist.
	makeRequest(test, "PUT", "/v2/collections/first/2", "", 200, "Track In Collection\n", false)
	makeRequest(test, "PUT", "/v2/collections/first/3", "", 200, "Track In Collection\n", false)
	makeRequest(test, "PUT", "/v2/collections/first/4", "", 404, "Track Not Found\n", false)
	makeRequest(test, "GET", "/v2/collections/first", "", 200, `{"slug":"first","name": "The Collection", "tracks":[{"fingerprint":"def2","duration":14,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"Dropkick Murphys", "title":"The Spicy McHaggis Jig"},"weighting": 0},{"fingerprint":"ghi3","duration":21,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Pachabel", "title":"Canon in D"},"weighting": 0}],"totalPages":1}`, true)
	restartServer()
	makeRequest(test, "GET", "/v2/collections/first/1", "", 404, "Track Not In Collection\n", false)
	makeRequest(test, "GET", "/v2/collections/first/2", "", 200, "Track In Collection\n", false)
	makeRequest(test, "GET", "/v2/collections/first/3", "", 200, "Track In Collection\n", false)
	makeRequest(test, "GET", "/v2/collections/first/4", "", 404, "Track Not Found\n", false)
	makeRequest(test, "GET", "/v2/tracks/1", "", 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"},"collections":[],"weighting": 0}`, true)
	makeRequest(test, "GET", "/v2/tracks/2", "", 200, `{"fingerprint":"def2","duration":14,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"Dropkick Murphys", "title":"The Spicy McHaggis Jig"},"collections":[{"slug":"first","name": "The Collection"}],"weighting": 0}`, true)
	makeRequest(test, "GET", "/v2/tracks/3", "", 200, `{"fingerprint":"ghi3","duration":21,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Pachabel", "title":"Canon in D"},"collections":[{"slug":"first","name": "The Collection"}],"weighting": 0}`, true)

	makeRequest(test, "DELETE", "/v2/collections/first/3", "", 200, "Track Not In Collection\n", false) // Feels slightly odd, but 200 is correct for having deleting something
	makeRequest(test, "DELETE", "/v2/collections/first/4", "", 404, "Track Not Found\n", false)
	makeRequest(test, "GET", "/v2/collections/first", "", 200, `{"slug":"first","name": "The Collection", "tracks":[{"fingerprint":"def2","duration":14,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"Dropkick Murphys", "title":"The Spicy McHaggis Jig"},"weighting": 0}],"totalPages":1}`, true)
	restartServer()
	makeRequest(test, "GET", "/v2/collections/first/1", "", 404, "Track Not In Collection\n", false)
	makeRequest(test, "GET", "/v2/collections/first/2", "", 200, "Track In Collection\n", false)
	makeRequest(test, "GET", "/v2/collections/first/3", "", 404, "Track Not In Collection\n", false)
	makeRequest(test, "GET", "/v2/collections/first/4", "", 404, "Track Not Found\n", false)
	makeRequest(test, "GET", "/v2/tracks/1", "", 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"},"collections":[],"weighting": 0}`, true)
	makeRequest(test, "GET", "/v2/tracks/2", "", 200, `{"fingerprint":"def2","duration":14,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"Dropkick Murphys", "title":"The Spicy McHaggis Jig"},"collections":[{"slug":"first","name": "The Collection"}],"weighting": 0}`, true)
	makeRequest(test, "GET", "/v2/tracks/3", "", 200, `{"fingerprint":"ghi3","duration":21,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Pachabel", "title":"Canon in D"},"collections":[],"weighting": 0}`, true)

	makeRequest(test, "DELETE", "/v2/collections/first/3", "", 200, "Track Not In Collection\n", false) // Feels really odd as track wasn't in collection.  But DELETE should be idempotent.
	makeRequest(test, "GET", "/v2/collections/first/3", "", 404, "Track Not In Collection\n", false)
	makeRequest(test, "GET", "/v2/collections/first", "", 200, `{"slug":"first","name": "The Collection", "tracks":[{"fingerprint":"def2","duration":14,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"Dropkick Murphys", "title":"The Spicy McHaggis Jig"},"weighting": 0}],"totalPages":1}`, true)
	makeRequest(test, "GET", "/v2/collections/second", "", 200, `{"slug":"second","name": "Another Collection", "tracks":[], "totalPages":0}`, true)  // No requests touched this collection, so should stay empty

	makeRequest(test, "PUT", "/v2/collections/third/2", "", 404, "Collection Not Found\n", false) // Collection 'third' doesn't exist.
	makeRequest(test, "PUT", "/v2/collections/third/4", "", 404, "Collection Not Found\n", false) // Neither Collection 'third' or Track 4 exist.  Erroring on collection as it comes first in URL

	makeRequest(test, "PUT", "/v2/collections/first/something", "", 404, "Collection Endpoint Not Found\n", false)

	makeRequestWithUnallowedMethod(test, "/v2/collections/first/2", "POST", []string{"PUT", "GET", "DELETE"})
}
func TestAddTracksToCollectionOnTracksEndpoint(test *testing.T) {
	clearData()
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc1", `{"url":"http://example.org/track1", "duration": 7,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"}}`, 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"},"collections":[],"weighting": 0}`, true)
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=def2", `{"url":"http://example.org/track2", "duration": 14,"tags":{"artist":"Dropkick Murphys", "title":"The Spicy McHaggis Jig"}}`, 200, `{"fingerprint":"def2","duration":14,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"Dropkick Murphys", "title":"The Spicy McHaggis Jig"},"collections":[],"weighting": 0}`, true)
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=ghi3", `{"url":"http://example.org/track3", "duration": 21,"tags":{"artist":"Pachabel", "title":"Canon in D"}}`, 200, `{"fingerprint":"ghi3","duration":21,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Pachabel", "title":"Canon in D"},"collections":[],"weighting": 0}`, true)
	makeRequest(test, "PUT", "/v2/collections/first", `{"name": "The Collection"}`, 200, `{"slug":"first","name": "The Collection", "tracks":[], "totalPages":0}`, true)
	makeRequest(test, "PUT", "/v2/collections/second", `{"name": "Another Collection"}`, 200, `{"slug":"second","name": "Another Collection", "tracks":[], "totalPages":0}`, true)

	restartServer()
	makeRequest(test, "PATCH", "/v2/tracks/1", `{"collections":[{"slug":"first"}]}`, 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"},"collections":[{"slug":"first","name": "The Collection"}],"weighting": 0}`, true)
	makeRequest(test, "PATCH", "/v2/tracks/2", `{"collections":[{"slug":"first"},{"slug":"second"}]}`, 200, `{"fingerprint":"def2","duration":14,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"Dropkick Murphys", "title":"The Spicy McHaggis Jig"},"collections":[{"slug":"first","name": "The Collection"},{"slug":"second","name": "Another Collection"}],"weighting": 0}`, true)
	makeRequest(test, "PATCH", "/v2/tracks/3", `{"collections":[{"slug":"first"},{"slug":"second"}]}`, 200, `{"fingerprint":"ghi3","duration":21,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Pachabel", "title":"Canon in D"},"collections":[{"slug":"first","name": "The Collection"},{"slug":"second","name": "Another Collection"}],"weighting": 0}`, true)
	assertEqual(test, "Loganne call", "trackUpdated", lastLoganneType)
	assertEqual(test, "Number of Loganne requests", 3, loganneRequestCount)
	restartServer()
	makeRequest(test, "GET", "/v2/tracks/1", "", 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"},"collections":[{"slug":"first","name": "The Collection"}],"weighting": 0}`, true)
	makeRequest(test, "GET", "/v2/tracks/2", "", 200, `{"fingerprint":"def2","duration":14,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"Dropkick Murphys", "title":"The Spicy McHaggis Jig"},"collections":[{"slug":"first","name": "The Collection"},{"slug":"second","name": "Another Collection"}],"weighting": 0}`, true)
	makeRequest(test, "GET", "/v2/tracks/3", "", 200, `{"fingerprint":"ghi3","duration":21,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Pachabel", "title":"Canon in D"},"collections":[{"slug":"first","name": "The Collection"},{"slug":"second","name": "Another Collection"}],"weighting": 0}`, true)

	makeRequest(test, "PATCH", "/v2/tracks/2", `{"collections":[{"slug":"second"}]}`, 200, `{"fingerprint":"def2","duration":14,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"Dropkick Murphys", "title":"The Spicy McHaggis Jig"},"collections":[{"slug":"second","name": "Another Collection"}],"weighting": 0}`, true)
	makeRequest(test, "GET", "/v2/collections/first", "", 200, `{"slug":"first","name": "The Collection", "tracks":[{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"},"weighting": 0},{"fingerprint":"ghi3","duration":21,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Pachabel", "title":"Canon in D"},"weighting": 0}],"totalPages":1}`, true)
	makeRequest(test, "GET", "/v2/collections/second", "", 200, `{"slug":"second","name": "Another Collection", "tracks":[{"fingerprint":"def2","duration":14,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"Dropkick Murphys", "title":"The Spicy McHaggis Jig"},"weighting": 0},{"fingerprint":"ghi3","duration":21,"url":"http://example.org/track3","trackid":3,"tags":{"artist":"Pachabel", "title":"Canon in D"},"weighting": 0}],"totalPages":1}`, true)

	restartServer()
	// Setting a track to the same collections as it already has, shouldn't update anything
	makeRequest(test, "PATCH", "/v2/tracks/2", `{"collections":[{"slug":"second"}]}`, 200, `{"fingerprint":"def2","duration":14,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"Dropkick Murphys", "title":"The Spicy McHaggis Jig"},"collections":[{"slug":"second","name": "Another Collection"}],"weighting": 0}`, true)
	assertEqual(test, "Number of Loganne requests", 0, loganneRequestCount) // No requests to loganne made since server restart

	// Create a new track with collections on it from the start
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=jkl4", `{"url":"http://example.org/track4", "duration": 28,"tags":{"artist":"Penguin Café Orchestra", "title":"Music for a found Harmonium"},"collections":[{"slug":"first"},{"slug":"second"}]}`, 200, `{"fingerprint":"jkl4","duration":28,"url":"http://example.org/track4","trackid":4,"tags":{"artist":"Penguin Café Orchestra", "title":"Music for a found Harmonium"},"collections":[{"slug":"first","name": "The Collection"},{"slug":"second","name": "Another Collection"}],"weighting": 0}`, true)
	assertEqual(test, "Loganne call", "trackAdded", lastLoganneType)
	assertEqual(test, "Number of Loganne requests", 1, loganneRequestCount)
}
func TestListCollections(test *testing.T) {
	clearData()
	makeRequest(test, "GET", "/v2/collections", "", 200, `[]`, true)
	makeRequest(test, "PUT", "/v2/collections/first", `{"name": "The Collection"}`, 200, `{"slug":"first","name": "The Collection", "tracks":[], "totalPages":0}`, true)
	makeRequest(test, "PUT", "/v2/collections/second", `{"name": "Another Collection"}`, 200, `{"slug":"second","name": "Another Collection", "tracks":[], "totalPages":0}`, true)
	makeRequest(test, "PUT", "/v2/collections/third", `{"name": "Collecty McCollectFace"}`, 200, `{"slug":"third","name": "Collecty McCollectFace", "tracks":[], "totalPages":0}`, true)
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc1", `{"url":"http://example.org/track1", "duration": 7,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"}}`, 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"},"collections":[],"weighting": 0}`, true)
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=def2", `{"url":"http://example.org/track2", "duration": 14,"tags":{"artist":"Dropkick Murphys", "title":"The Spicy McHaggis Jig"}}`, 200, `{"fingerprint":"def2","duration":14,"url":"http://example.org/track2","trackid":2,"tags":{"artist":"Dropkick Murphys", "title":"The Spicy McHaggis Jig"},"collections":[],"weighting": 0}`, true)
	makeRequest(test, "PUT", "/v2/collections/first/1", "", 200, "Track In Collection\n", false)
	makeRequest(test, "PUT", "/v2/collections/first/2", "", 200, "Track In Collection\n", false)
	makeRequest(test, "PUT", "/v2/collections/third/2", "", 200, "Track In Collection\n", false)
	restartServer()
	makeRequest(test, "GET", "/v2/collections", "", 200, `[{"slug":"first","name": "The Collection", "totalTracks":2, "totalPages":1},{"slug":"second","name": "Another Collection", "totalTracks":0, "totalPages":0},{"slug":"third","name": "Collecty McCollectFace", "totalTracks":1, "totalPages":1}]`, true)
	makeRequest(test, "GET", "/v2/collections/", "", 200, `[{"slug":"first","name": "The Collection", "totalTracks":2, "totalPages":1},{"slug":"second","name": "Another Collection", "totalTracks":0, "totalPages":0},{"slug":"third","name": "Collecty McCollectFace", "totalTracks":1, "totalPages":1}]`, true)

}

/**
 * Checks the pagination of the collections endpoint
 */
func TestCollectionPagination(test *testing.T) {
	clearData()
	makeRequest(test, "PUT", "/v2/collections/bigone", `{"name": "Large Collection"}`, 200, `{"slug":"bigone","name": "Large Collection", "tracks":[], "totalPages":0}`, true)

	// Create 45 Tracks in the collection
	for i := 1; i <= 45; i++ {
		id := strconv.Itoa(i)
		trackurl := "http://example.org/track/id" + id
		escapedTrackUrl := url.QueryEscape(trackurl)
		trackpath := fmt.Sprintf("/v2/tracks?url=%s", escapedTrackUrl)
		inputJson := `{"fingerprint": "abcde` + id + `", "duration": 350, "collections":[{"slug":"bigone"}]}`
		outputJson := `{"fingerprint": "abcde` + id + `", "duration": 350, "url": "` + trackurl + `", "trackid": ` + id + `, "tags": {},"collections":[{"slug":"bigone", "name": "Large Collection"}], "weighting": 0}`
		makeRequest(test, "PUT", trackpath, inputJson, 200, outputJson, true)
		makeRequest(test, "PUT", "/v2/tracks/"+id+"/weighting", "4.3", 200, "4.3", false)
	}
	pages := map[string]int{
		"1": 20,
		"2": 20,
		"3": 5,
	}
	for page, count := range pages {

		path := "/v2/collections/bigone?page=" + page
		request := basicRequest(test, "GET", path, "")
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			test.Error(err)
		}
		responseData, err := ioutil.ReadAll(response.Body)
		if err != nil {
			test.Error(err)
		}

		expectedResponseCode := 200
		if response.StatusCode != expectedResponseCode {
			test.Errorf("Got response code %d, expected %d for %s", response.StatusCode, expectedResponseCode, path)
		}

		var output Collection
		err = json.Unmarshal(responseData, &output)
		if err != nil {
			test.Errorf("Invalid JSON body: %s for %s", err.Error(), "/v2/collections/bigone?page="+page)
		}
		if len(*output.Tracks) != count {
			test.Errorf("Wrong number of tracks.  Expected: %d, Actual: %d", count, len(*output.Tracks))
		}
		if *output.TotalPages != 3 {
			test.Errorf("Wrong number of totalPages.  Expected: 3, Actual: %d", output.TotalPages)
		}
	}
}
func TestReservedCollectionSlugs(test *testing.T) {
	clearData()
	makeRequest(test, "PUT", "/v2/collections/all", `{"name": "Mega Collection"}`, 400, "Collection with slug all not allowed\n", false)
	makeRequest(test, "PUT", "/v2/collections/new", `{"name": "Shiney Collection"}`, 400, "Collection with slug new not allowed\n", false)
	makeRequest(test, "PUT", "/v2/collections/collection", `{"name": "Meta Collection"}`, 400, "Collection with slug collection not allowed\n", false)
}
func TestDeleteCollection(test *testing.T) {
	clearData()
	makeRequest(test, "PUT", "/v2/collections/first", `{"name": "The Collection"}`, 200, `{"slug":"first","name": "The Collection", "tracks":[], "totalPages":0}`, true)
	makeRequest(test, "PUT", "/v2/collections/second", `{"name": "Another Collection"}`, 200, `{"slug":"second","name": "Another Collection", "tracks":[], "totalPages":0}`, true)
	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc1", `{"url":"http://example.org/track1", "duration": 7,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"}}`, 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"},"collections":[],"weighting": 0}`, true)
	makeRequest(test, "PUT", "/v2/collections/first/1", "", 200, "Track In Collection\n", false)
	makeRequest(test, "GET", "/v2/collections/first", "", 200, `{"slug":"first","name": "The Collection", "tracks":[{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"},"weighting": 0}],"totalPages":1}`, true)
	restartServer()
	makeRequest(test, "DELETE", "/v2/collections/first", "", 204, "", false)
	assertEqual(test, "Loganne event type", "collectionDeleted", lastLoganneType)
	assertEqual(test, "Loganne humanReadable", "Collection \"The Collection\" deleted", lastLoganneMessage)
	assertEqual(test, "Loganne updatedCollection slug", "", lastLoganneUpdatedCollection.Slug)
	assertEqual(test, "Loganne existingCollection slug", "first", lastLoganneExistingCollection.Slug)
	makeRequest(test, "GET", "/v2/collections/first", "", 404, "Collection Not Found\n", false)
	makeRequest(test, "GET", "/v2/collections/first/1", "", 404, "Collection Not Found\n", false)
	makeRequest(test, "GET", "/v2/collections/second", "", 200, `{"slug":"second","name": "Another Collection", "tracks":[], "totalPages":0}`, true)

}
func TestCollectionPutWithNoData(test *testing.T) {
	clearData()
	makeRequest(test, "PUT", "/v2/collections/first", "", 400, "No Data Sent\n", false)
}

/**
 * Checks random tracks endpoint of a collection
 */
func TestCollectionRandomTracks(test *testing.T) {
	clearData()
	path := "/v2/collections/rand-coll/random"
	makeRequest(test, "PUT", "/v2/collections/rand-coll", `{"name": "Random Collection"}`, 200, `{"slug":"rand-coll","name": "Random Collection", "tracks":[], "totalPages":0}`, true)

	// Check the random function when the collection is empty
	makeRequest(test, "GET", path, "", 200, `{"slug":"rand-coll","name": "Random Collection", "tracks":[], "totalPages":0}`, true)

	// Create 40 Tracks
	for i := 1; i <= 40; i++ {
		id := strconv.Itoa(i)
		trackurl := "http://example.org/track/id" + id
		escapedTrackUrl := url.QueryEscape(trackurl)
		trackpath := fmt.Sprintf("/v2/tracks?url=%s", escapedTrackUrl)
		inputJson := `{"fingerprint": "abcde` + id + `", "duration": 350, "collections":[{"slug":"rand-coll"}]}`
		outputJson := `{"fingerprint": "abcde` + id + `", "duration": 350, "url": "` + trackurl + `", "trackid": ` + id + `, "tags": {},"collections":[{"slug":"rand-coll","name": "Random Collection"}], "weighting": 0}`
		makeRequest(test, "PUT", trackpath, inputJson, 200, outputJson, true)
		makeRequest(test, "PUT", "/v2/tracks/"+id+"/weighting", "4.3", 200, "4.3", false)
	}
	request := basicRequest(test, "GET", path, "")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		test.Error(err)
	}
	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		test.Error(err)
	}

	expectedResponseCode := 200
	if response.StatusCode != expectedResponseCode {
		test.Errorf("Got response code %d, expected %d for %s", response.StatusCode, expectedResponseCode, path)
	}

	var output SearchResult
	err = json.Unmarshal(responseData, &output)
	if err != nil {
		test.Errorf("Invalid JSON body: %s for %s", err.Error(), path)
	}
	if len(output.Tracks) != 20 {
		test.Errorf("Wrong number of tracks.  Expected: 20, Actual: %d", len(output.Tracks))
	}
	if output.TotalPages != 1 {
		test.Errorf("Wrong number of totalPages.  Expected: 1, Actual: %d", output.TotalPages)
	}
	if !strings.Contains(response.Header.Get("Cache-Control"), "no-cache") {
		test.Errorf("Random track list is cachable")
	}
	if !strings.Contains(response.Header.Get("Cache-Control"), "max-age=0") {
		test.Errorf("Random track missing max-age of 0")
	}
}

/**
 * Checks random tracks endpoint when a collection isn't large enough to have 20 tracks
 */
func TestSmallCollectionRandomTracks(test *testing.T) {
	clearData()
	makeRequest(test, "PUT", "/v2/collections/rand-coll", `{"name": "Random Collection"}`, 200, `{"slug":"rand-coll","name": "Random Collection", "tracks":[], "totalPages":0}`, true)

	makeRequest(test, "PUT", "/v2/tracks?fingerprint=abc1", `{"url":"http://example.org/track1", "duration": 7,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"}, "collections":[{"slug":"rand-coll"}]}`, 200, `{"fingerprint":"abc1","duration":7,"url":"http://example.org/track1","trackid":1,"tags":{"artist":"The Beatles", "title":"Yellow Submarine"},"collections":[{"slug":"rand-coll","name": "Random Collection"}],"weighting": 0}`, true)
	makeRequest(test, "PUT", "/v2/tracks/1/weighting", "0.7", 200, "0.7", false)

	path := "/v2/collections/rand-coll/random"
	request := basicRequest(test, "GET", path, "")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		test.Error(err)
	}
	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		test.Error(err)
	}

	expectedResponseCode := 200
	if response.StatusCode != expectedResponseCode {
		test.Errorf("Got response code %d, expected %d for %s", response.StatusCode, expectedResponseCode, path)
	}

	var output SearchResult
	err = json.Unmarshal(responseData, &output)
	if err != nil {
		test.Errorf("Invalid JSON body: %s for %s", err.Error(), path)
	}
	if len(output.Tracks) != 20 {
		test.Errorf("Wrong number of tracks.  Expected: 20, Actual: %d", len(output.Tracks))
	}
	if output.TotalPages != 1 {
		test.Errorf("Wrong number of totalPages.  Expected: 1, Actual: %d", output.TotalPages)
	}
	if !strings.Contains(response.Header.Get("Cache-Control"), "no-cache") {
		test.Errorf("Random track list is cachable")
	}
	if !strings.Contains(response.Header.Get("Cache-Control"), "max-age=0") {
		test.Errorf("Random track missing max-age of 0")
	}
}