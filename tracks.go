package main

import (
	"io"
	"net/http"
	"encoding/json"
	"path"
)

/**
 * A struct for holding data about a given track
 */
type Track struct {
	Fingerprint string `json:"fingerprint"`
	Duration    int `json:"duration"`
	URL         string `json:"url"`
	ID          int `json:"trackid"`
}

/**
 * Updates data about a track for a given URL
 *
 */
func (store Datastore) updateTrackDataByURL(trackurl string, track Track) (err error) {
	trackExists, _, err := store.getTrackDataByURL(trackurl)
	if err != nil {
		return err
	}
	query := ""
	if (trackExists) {
		query = "UPDATE TRACK SET fingerprint = ?, duration = ? WHERE url = ?"
	} else {
		query = "INSERT INTO track(fingerprint, duration, url) values(?, ?, ?)"
	}
	stmt, err := store.DB.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(track.Fingerprint, track.Duration, trackurl)
	return err
}

/**
 * Updates data about a track for a given trackid
 *
 */
func (store Datastore) updateTrackDataByID(trackid string, track Track) (err error) {
	stmt, err := store.DB.Prepare("UPDATE TRACK SET fingerprint = ?, duration = ?, url = ? WHERE id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(track.Fingerprint, track.Duration, track.URL, trackid)
	return err
}

/**
 * Gets data about a track for a given URL
 *
 */
func (store Datastore) getTrackDataByURL(trackurl string) (found bool, track *Track, err error) {
	found = false
	track = new(Track)
	stmt, err := store.DB.Prepare("SELECT id, url, fingerprint, duration FROM track WHERE url = ?")
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
	found = true
	err = rows.Scan(&track.ID, &track.URL, &track.Fingerprint, &track.Duration)
	if err != nil {
		return
	}
	return
}

/**
 * Gets data about a track for a given ID
 *
 */
func (store Datastore) getTrackDataByID(trackid string) (found bool, track *Track, err error) {
	found = false
	track = new(Track)
	stmt, err := store.DB.Prepare("SELECT id, url, fingerprint, duration FROM track WHERE id = ?")
	if err != nil {
		return
	}
	defer stmt.Close()
	rows, err := stmt.Query(trackid)
	if err != nil {
		return
	}
	defer rows.Close()

	result := rows.Next()
	if (result == false) {
		return
	}
	found = true
	err = rows.Scan(&track.ID, &track.URL, &track.Fingerprint, &track.Duration)
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
	rows, err := store.DB.Query("SELECT id, url, fingerprint, duration FROM track")
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		track := new(Track)
		err = rows.Scan(&track.ID, &track.URL, &track.Fingerprint, &track.Duration)
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
	writeResponse(w, true, tracks, err)
}

/**
 * Writes a http response with a JSON representation of a given track
 */
func writeTrackDataByURL(store Datastore, w http.ResponseWriter, trackurl string) {
	trackfound, track, err := store.getTrackDataByURL(trackurl)
	writeResponse(w, trackfound, track, err)
}
/**
 * Writes a http response with a JSON representation of a given track
 */
func writeTrackDataByID(store Datastore, w http.ResponseWriter, trackid string) {
	trackfound, track, err := store.getTrackDataByID(trackid)
	writeResponse(w, trackfound, track, err)
}

/**
 * Writes a http response based on some data
 */
func writeResponse(w http.ResponseWriter, found bool, data interface{}, err error) {
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if (!found) {
		http.Error(w, "Track Not Found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
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
				err = store.updateTrackDataByURL(trackurl, track)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				fallthrough
			case "GET":
				writeTrackDataByURL(store, w, trackurl)
			default:
				MethodNotAllowed(w, []string{"GET", "PUT"})
		}
	}
}

/** 
 * A controller for handling all requests dealing with tracks
 */
func (store Datastore) DenormTracksController(w http.ResponseWriter, r *http.Request) {
	trackid := path.Base(r.URL.Path)
	switch r.Method {
		case "PUT":
			track, err := DecodeTrack(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			err = store.updateTrackDataByID(trackid, track)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			fallthrough
		case "GET":
			writeTrackDataByID(store, w, trackid)
		default:
			MethodNotAllowed(w, []string{"GET", "PUT"})
	}
}