package main

import (
	"io"
	"net/http"
	"encoding/json"
	"path"
	"strconv"
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
 * Updates data about a track for a given URL
 *
 */
func (store Datastore) updateTrackDataByURL(trackurl string, track Track) (err error) {
	trackExists, _, err := store.getTrackDataByField("url", trackurl)
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
 * Updates data about a track for a given Fingerprint
 *
 */
func (store Datastore) updateTrackDataByFingerprint(fingerprint string, track Track) (err error) {
	trackExists, _, err := store.getTrackDataByField("fingerprint", fingerprint)
	if err != nil {
		return err
	}
	query := ""
	if (trackExists) {
		query = "UPDATE TRACK SET duration = ?, url = ? WHERE fingerprint = ?"
	} else {
		query = "INSERT INTO track(duration, url, fingerprint) values(?, ?, ?)"
	}
	stmt, err := store.DB.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(track.Duration, track.URL, fingerprint)
	return err
}

/**
 * Updates data about a track for a given trackid
 *
 */
func (store Datastore) updateTrackDataByID(trackid int, track Track) (err error) {
	stmt, err := store.DB.Prepare("UPDATE TRACK SET fingerprint = ?, duration = ?, url = ? WHERE id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(track.Fingerprint, track.Duration, track.URL, trackid)
	return err
}

/**
 * Gets data about a track for a given value of a given field
 *
 */
func (store Datastore) getTrackDataByField(field string, value interface{}) (found bool, track *Track, err error) {
	found = false
	track = new(Track)
	stmt, err := store.DB.Prepare("SELECT id, url, fingerprint, duration FROM track WHERE "+field+" = ?")
	if err != nil {
		return
	}
	defer stmt.Close()
	rows, err := stmt.Query(value)
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
	track.Tags, err = store.getAllTagsForTrack(track.ID)
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
		track.Tags, err = store.getAllTagsForTrack(track.ID)
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
	if (len(trackurl) > 0) {
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
				writeTrackDataByField(store, w, "url", trackurl)
			default:
				MethodNotAllowed(w, []string{"GET", "PUT"})
		}
	} else if (len(fingerprint) > 0) {
		switch r.Method {
			case "PUT":
				track, err := DecodeTrack(r.Body)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				err = store.updateTrackDataByFingerprint(fingerprint, track)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				fallthrough
			case "GET":
				writeTrackDataByField(store, w, "fingerprint", fingerprint)
			default:
				MethodNotAllowed(w, []string{"GET", "PUT"})
		}
	} else {
		if (r.Method == "GET") {
			writeAllTrackData(store, w)
		} else {
			MethodNotAllowed(w, []string{"GET"})
		}
	}
}

/** 
 * A controller for handling all requests dealing with denorm tracks
 */
func (store Datastore) DenormTracksController(w http.ResponseWriter, r *http.Request) {
	trackid, err := strconv.Atoi(path.Base(r.URL.Path))
	if err != nil {
		http.Error(w, "Track ID must be an integer", http.StatusBadRequest)
		return
	}
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
			writeTrackDataByField(store, w, "id", trackid)
		default:
			MethodNotAllowed(w, []string{"GET", "PUT"})
	}
}