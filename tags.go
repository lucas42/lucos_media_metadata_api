package main

import (
	"errors"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
)

type Tag struct {
	TrackID     int
	PredicateID string
	Value       string
}

/**
 * Get the value for a given tag
 *
 */
func (store Datastore) getTagValue(trackid int, predicate string) (value string, err error) {
	err = store.DB.Get(&value, "SELECT value FROM tag WHERE trackid = $1 AND predicateid = $2", trackid, predicate)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			err = errors.New("Tag Not Found")
		}
	}
	return
}

/**
 * Creates or updates a tag
 *
 */
func (store Datastore) updateTag(trackid int, predicate string, value string) (err error) {
	trackFound, err := store.trackExists("id", trackid)
	if err != nil {
		return
	}
	if !trackFound {
		err = errors.New("Unknown Track")
		return
	}
	hasPredicate, err := store.hasPredicate(predicate)
	if err != nil {
		return
	}
	if !hasPredicate {
		err = store.createPredicate(predicate)
		if err != nil {
			return
		}
	}
	_, err = store.DB.Exec("REPLACE INTO tag(trackid, predicateid, value) values($1, $2, $3)", trackid, predicate, value)
	return
}

/**
 * Gets all the tags for a given track (in a map of key/value pairs)
 *
 */
func (store Datastore) getAllTagsForTrack(trackid int) (tags map[string]string, err error) {
	tags = make(map[string]string)
	tagList := []Tag{}
	err = store.DB.Select(&tagList, "SELECT * FROM tag WHERE trackid = ?", trackid)
	if err != nil {
		return
	}
	for _, tag := range tagList {
		tags[tag.PredicateID] = tag.Value
	}
	return
}

/**
 * Writes a http response with the value of a tag
 */
func writeTag(store Datastore, w http.ResponseWriter, trackid int, predicate string) {
	value, err := store.getTagValue(trackid, predicate)
	writePlainResponse(w, value, err)
}

/**
 * Writes a http response with a JSON representation of all tags for a given track
 */
func writeAllTagsForTrack(store Datastore, w http.ResponseWriter, trackid int) {
	tags, err := store.getAllTagsForTrack(trackid)
	writeJSONResponse(w, tags, err)
}

/**
 * A controller for handling all requests dealing with tags
 */
func (store Datastore) TagsController(w http.ResponseWriter, r *http.Request) {
	pathparts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathparts) < 2 {
		http.Redirect(w, r, "/tracks", http.StatusFound)
		return
	}
	trackid, err := strconv.Atoi(pathparts[1])
	if err != nil {
		http.Error(w, "Track ID must be an integer", http.StatusBadRequest)
		return
	}
	if len(pathparts) < 3 {
		if r.Method == "GET" {
			writeAllTagsForTrack(store, w, trackid)
		} else {
			MethodNotAllowed(w, []string{"GET"})
		}
		return
	}
	predicate := pathparts[2]
	switch r.Method {
	case "PUT":
		value, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if r.Header.Get("If-None-Match") == "*" {
			_, err = store.getTagValue(trackid, predicate)
			if err != nil && err.Error() == "Tag Not Found" {
				err = store.updateTag(trackid, predicate, string(value))
			}
		} else {
			err = store.updateTag(trackid, predicate, string(value))
		}
		if err != nil {
			var status int
			if err.Error() == "Unknown Track" {
				status = http.StatusNotFound
			} else {
				status = http.StatusInternalServerError
			}
			http.Error(w, err.Error(), status)
			return
		}
		fallthrough
	case "GET":
		writeTag(store, w, trackid, predicate)
	default:
		MethodNotAllowed(w, []string{"GET", "PUT"})
	}
}
