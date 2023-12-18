package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

type Collection struct {
	Slug   string  `json:"slug"`
	Name   string  `json:"name"`
	Tracks []Track `json:"tracks"`
}


/**
 * Gets data about a track for a given value of a given field
 *
 */
func (store Datastore) getCollection(slug string) (collection Collection, err error) {
	collection = Collection{}
	err = store.DB.Get(&collection, "SELECT slug, name FROM collection WHERE slug=$1", slug)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			err = errors.New("Collection Not Found")
		}
		return
	}
	collection.Tracks, err = store.getTracksInCollection(collection.Slug)
	return
}



/**
 * Gets all the tags for a given track (in a map of key/value pairs)
 *
 */
func (store Datastore) getTracksInCollection(slug string) (tracks []Track, err error) {
	tracks = []Track{}
	err = store.DB.Select(&tracks, "SELECT id, url, fingerprint, duration, weighting FROM collection_track LEFT JOIN track ON collection_track.trackid = track.id WHERE collection_track.collectionslug = $1", slug)
	if err != nil {
		return
	}

	// Loop through all the tracks and add tags for each one
	for i := range tracks {
		track := &tracks[i]
		track.Tags, err = store.getAllTagsForTrack(track.ID)
		if err != nil {
			return
		}
	}
	return
}

func (original Collection) updateNeeded(changeSet Collection) bool {
	if changeSet.Name != "" && changeSet.Name != original.Name {
		return true
	}
	// TODO: check for different tracks in collection
	return false
}

/**
 * Checks whether any other collections are duplicating a given field
 */
func (store Datastore) checkForDuplicateCollection(compareField string, compareValue interface{}, filterField string, filterValue interface{}) (err error) {
	var slug string
	err = store.DB.Get(&slug, "SELECT slug FROM collection WHERE "+compareField+" = $1 AND "+filterField+" != $2", compareValue, filterValue)
	if err != nil && err.Error() == "sql: no rows in result set" {
		err = nil
	}
	if (slug != "") {
		err = errors.New("Duplicate: collection "+slug+" has same "+compareField)
	}
	return
}

/**
 * Updates or Creates a collection
 *
 */
func (store Datastore) updateCreateCollection(existingCollection Collection, newCollection Collection) (storedCollection Collection, action string, err error) {
	action = "noChange"
	storedCollection = existingCollection
	if newCollection.Tracks == nil {
		newCollection.Tracks = []Track{}
	}

	// If no changes are needed, return the existing collection
	if !existingCollection.updateNeeded(newCollection) {
		return
	}
	action = "Changed"
	if newCollection.Name != "" {
		err = store.checkForDuplicateCollection("name", newCollection.Name, "slug", newCollection.Slug)
		if err != nil {
			return
		}
		if existingCollection.Slug != "" {
			_, err = store.DB.NamedExec("UPDATE collection SET name = :name WHERE slug = :slug", newCollection)
			storedCollection.Name = newCollection.Name
		} else {
			_, err = store.DB.NamedExec("INSERT INTO collection(slug, name) values(:slug, :name)", newCollection)
			storedCollection = newCollection
		}
		if err != nil {
			return
		}
	}
	// TODO: add/remove tracks
	if existingCollection.Slug != "" {
		action = "collectionUpdated"
		store.Loganne.collectionPost(action, "Music Collection "+newCollection.Name+" Updated", newCollection, existingCollection)
	} else {
		action = "collectionCreated"
		store.Loganne.collectionPost(action, "New Music Collection Created: "+newCollection.Name, newCollection, existingCollection)
	}
	return
}

/**
 * Decodes a JSON representation of a collection
 */
func DecodeCollection(r io.Reader) (Collection, error) {
	collection := new(Collection)
	err := json.NewDecoder(r).Decode(collection)
	return *collection, err
}

/**
 * A controller for handling requests dealing with collections
 */
func (store Datastore) CollectionsV2Controller(w http.ResponseWriter, r *http.Request) {
	normalisedpath := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v2/collections"), "/")
	pathparts := strings.Split(normalisedpath, "/")

	if len(pathparts) <= 1 {
		writeErrorResponse(w, errors.New("Base Endpoint Not Implemented"))
	} else {
		slug := pathparts[1]
		if len(pathparts) == 2 {
			collection, err := store.getCollection(slug)
			if err != nil && r.Method == "PUT" && err.Error() == "Collection Not Found" {
				err = nil
			}
			if err != nil {
				writeErrorResponse(w, err)
				return
			}
			switch r.Method {
			case "DELETE":
				writeErrorResponse(w, errors.New("DELETE Not Implemented"))
			case "PUT":
				var newCollection Collection
				newCollection, err = DecodeCollection(r.Body)
				if err != nil {
					writeErrorResponse(w, err)
					return
				}
				newCollection.Slug = slug
				collection, _, err = store.updateCreateCollection(collection, newCollection)
				fallthrough
			case "GET":
				writeJSONResponse(w, collection, err)
			default:
				MethodNotAllowed(w, []string{"GET", "PUT", "DELETE"})
			}
		} else {
			writeErrorResponse(w, errors.New("Sub Endpoint Not Implemented"))
		}
	}
}