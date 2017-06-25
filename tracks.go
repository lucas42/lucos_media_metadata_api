package main

import (
	"io"
	"net/http"
	"encoding/json"
)

/**
 * A struct for holding data about a given track
 */
type Track struct {
	Fingerprint string `json:"fingerprint"`
	Duration    int `json:"duration"`
	URL         string `json:"url"`
}

/**
 * Updates data about a track for a given URL
 *
 */
func (store Datastore) updateTrackData(trackurl string, track Track) (err error) {
	stmt, err := store.DB.Prepare("REPLACE INTO track(url, fingerprint, duration) values(?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(trackurl, track.Fingerprint, track.Duration)
	return err
}

/**
 * Gets data about a track for a given URL
 *
 */
func (store Datastore) getTrackData(trackurl string) (track *Track, err error) {
	track = new(Track)
	stmt, err := store.DB.Prepare("SELECT url, fingerprint, duration FROM track WHERE url = ?")
	if err != nil {
		return
	}
	defer stmt.Close()
	rows, err := stmt.Query(trackurl)
	if err != nil {
		return
	}
	defer rows.Close()

	result := rows.Next()
	if (result == false) {
		return
	}
	err = rows.Scan(&track.URL, &track.Fingerprint, &track.Duration)
	if err != nil {
		return
	}
	return
}

/**
 * Gets data about all tracks in the database
 *
 */
func (store Datastore) getAllTracks() (tracks []*Track, err error) {
	tracks = []*Track{}
	rows, err := store.DB.Query("SELECT url, fingerprint, duration FROM track")
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		track := new(Track)
		err = rows.Scan(&track.URL, &track.Fingerprint, &track.Duration)
		if err != nil {
			return
		}
		tracks = append(tracks, track)
	}
	err = rows.Err()
	if err != nil {
		return
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
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(tracks)
}

/**
 * Writes a http response with a JSON representation of a given track
 */
func writeTrackData(store Datastore, w http.ResponseWriter, trackurl string) {
	track, err := store.getTrackData(trackurl)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if (track.URL != trackurl) {
		http.Error(w, "Track Not Found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(track)
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
	if (len(trackurl) == 0) {
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
				err = store.updateTrackData(trackurl, track)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				fallthrough
			case "GET":
				writeTrackData(store, w, trackurl)
			default:
				MethodNotAllowed(w, []string{"GET", "PUT"})
		}
	}
}