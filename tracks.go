package main

import (
	"io"
	"net/http"
	"encoding/json"
	"strconv"
	"strings"
)

/**
 * A struct for holding data about a given track
 */
type Track struct {
	Fingerprint string `json:"fingerprint"`
	Duration    int `json:"duration"`
	URL         string `json:"url"`
	ID          int `json:"trackid"`
	Tags        map[string]string `json:"tags"`
}

/**
 * Updates data about a track based on a given field
 *
 */
func (store Datastore) updateTrackDataByField(field string, value interface{}, track Track) (err error) {
	trackExists, _, err := store.getTrackDataByField(field, value)
	if err != nil { return }
	if (trackExists) {
		_, err = store.DB.NamedExec("UPDATE TRACK SET duration = :duration, url = :url, fingerprint = :fingerprint WHERE "+field+" = :"+field, track)
	} else {
		_, err = store.DB.NamedExec("INSERT INTO track(duration, url, fingerprint) values(:duration, :url, :fingerprint)", track)
	}
	return
}

/**
 * Gets data about a track for a given value of a given field
 *
 */
func (store Datastore) getTrackDataByField(field string, value interface{}) (found bool, track Track, err error) {
	found = true
	track = Track{}
    err = store.DB.Get(&track, "SELECT id, url, fingerprint, duration FROM track WHERE "+field+"=$1", value)
    if err != nil {
    	if (err.Error() == "sql: no rows in result set") { 
    		found = false
    		err = nil
    	}
    	return
    }
	track.Tags, err = store.getAllTagsForTrack(track.ID)
	return
}


/**
 * Gets data about all tracks in the database
 *
 */
func (store Datastore) getAllTracks() (tracks []Track, err error) {
	tracks = []Track{}
	err = store.DB.Select(&tracks, "SELECT id, url, fingerprint, duration FROM track")
	if (err != nil) { return }

	// Loop through all the tracks and add tags for each one
	for i := range tracks {
		track := &tracks[i]
		track.Tags, err = store.getAllTagsForTrack(track.ID)
		if (err != nil) { return }
	}
	return
}

/**
 * Write a http response with a JSON representation of all tracks
 *
 * TODO: pagination
 */
func writeAllTrackData(store Datastore, w http.ResponseWriter) {
	tracks, err := store.getAllTracks()
	writeResponse(w, true, "Track", tracks, err)
}

/**
 * Writes a http response with a JSON representation of a given track
 */
func writeTrackDataByField(store Datastore, w http.ResponseWriter, field string, value interface{}) {
	trackfound, track, err := store.getTrackDataByField(field, value)
	writeResponse(w, trackfound, "Track", track, err)
}

/**
 * Decodes a JSON representation of a track
 */
func DecodeTrack(r io.Reader) (Track, error) {
    track := new(Track)
    err := json.NewDecoder(r).Decode(track)
    return *track, err
}

/** 
 * A controller for handling all requests dealing with tracks
 */
func (store Datastore) TracksController(w http.ResponseWriter, r *http.Request) {
	trackurl := r.URL.Query().Get("url")
	fingerprint := r.URL.Query().Get("fingerprint")
	pathparts := strings.Split(strings.Trim(r.URL.Path,"/"), "/")

	var trackid int
	var filtervalue interface{}
	var filterfield string
	if (len(pathparts) > 1) {
		id, err := strconv.Atoi(pathparts[1])
		if err != nil {
			http.Error(w, "Track ID must be an integer", http.StatusBadRequest)
			return
		}
		filterfield = "id"
		filtervalue = pathparts[1]
		trackid = id
	} else if (len(trackurl) > 0) {
		filterfield = "url"
		filtervalue = trackurl
	} else if (len(fingerprint) > 0) {
		filterfield = "fingerprint"
		filtervalue = fingerprint
	}
	if (filterfield == "") {
		if (r.Method == "GET") {
			writeAllTrackData(store, w)
		} else {
			MethodNotAllowed(w, []string{"GET"})
		}
	} else {
		switch r.Method {
			case "PUT":
				track, err := DecodeTrack(r.Body)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				switch filterfield {
					case "id":
						track.ID = trackid
					case "url":
						track.URL = trackurl
					case "fingerprint":
						track.Fingerprint = fingerprint
				}
				err = store.updateTrackDataByField(filterfield, filtervalue, track)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				fallthrough
			case "GET":
				writeTrackDataByField(store, w, filterfield, filtervalue)
			default:
				MethodNotAllowed(w, []string{"GET", "PUT"})
		}
	}
}
