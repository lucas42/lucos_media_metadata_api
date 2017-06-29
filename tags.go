package main

import (
	"errors"
	"io/ioutil"
	"net/http"
	"strings"
)

/**
 * Get the value for a given tag
 *
 */
func (store Datastore) getTagValue(trackid string, predicate string) (found bool, value string, err error) {
	found = false
	stmt, err := store.DB.Prepare("SELECT value FROM tag WHERE trackid = ? AND predicateid = ?")
	if err != nil {
		return
	}
	defer stmt.Close()
	rows, err := stmt.Query(trackid, predicate)
	if err != nil {
		return
	}
	defer rows.Close()

	result := rows.Next()
	if (result == false) {
		return
	}
	found = true
	err = rows.Scan(&value)
	if err != nil {
		return
	}
	return
}

/**
 * Creates or updates a tag
 *
 */
func (store Datastore) updateTag(trackid string, predicate string, value string) (err error) {
	trackFound, _ , err := store.getTrackDataByID(trackid)
	if err != nil { return }
	if (!trackFound) {
		err = errors.New("Unknown Track")
		return
	}
	hasPredicate, err := store.hasPredicate(predicate)
	if err != nil { return }
	if (!hasPredicate) {
		err = store.createPredicate(predicate)
		if err != nil { return }
	}
	stmt, err := store.DB.Prepare("REPLACE INTO tag(trackid, predicateid, value) values(?,?,?)")
	if err != nil { return }
	defer stmt.Close()
	_, err = stmt.Exec(trackid, predicate, value)
	return err
}

/**
 * Gets all the tags for a given track
 *
 */
func (store Datastore) getAllTagsForTrack(trackid string) (tags map[string]string, err error) {
	tags = make(map[string]string)
	stmt, err := store.DB.Prepare("SELECT predicateid, value FROM tag WHERE trackid = ?")
	if err != nil {
		return
	}
	defer stmt.Close()
	rows, err := stmt.Query(trackid)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var predicateid string
		var value string
		err = rows.Scan(&predicateid, &value)
		if err != nil {
			return
		}
		tags[predicateid] = value
	}
	err = rows.Err()
	if err != nil {
		return
	}
	return
}


/**
 * Writes a http response with the value of a tag
 */ 
func writeTag(store Datastore, w http.ResponseWriter, trackid string, predicate string) {
	found, value, err := store.getTagValue(trackid, predicate)
	writePlainResponse(w, found, "Tag", value, err)
}

/**
 * Writes a http response with a JSON representation of all tags for a given track
 */
func writeAllTagsForTrack(store Datastore, w http.ResponseWriter, trackid string) {
	tags, err := store.getAllTagsForTrack(trackid)
	writeResponse(w, true, "Tag", tags, err)
}

/** 
 * A controller for handling all requests dealing with tags
 */
func (store Datastore) TagsController(w http.ResponseWriter, r *http.Request) {
	pathparts := strings.Split(strings.Trim(r.URL.Path,"/"), "/")
	if (len(pathparts) < 2) {
		http.Redirect(w, r, "/tracks", http.StatusFound)
		return
	}
	trackid := pathparts[1]
	if (len(pathparts) < 3) {
		if (r.Method == "GET") {
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
			bypassUpdate := false
			if (r.Header.Get("If-None-Match") == "*") {
				bypassUpdate, _, err = store.getTagValue(trackid, predicate)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
			if (!bypassUpdate) {
				err = store.updateTag(trackid, predicate, string(value))
				if err != nil {
					var status int
					if (err.Error() == "Unknown Track") {
						status = http.StatusNotFound
					} else {
						status = http.StatusInternalServerError
					}
					http.Error(w, err.Error(), status)
					return
				}
			}
			fallthrough
		case "GET":
			writeTag(store, w, trackid, predicate)
		default:
			MethodNotAllowed(w, []string{"GET", "PUT"})
	}
}