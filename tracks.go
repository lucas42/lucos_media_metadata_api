package main

import (
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"errors"
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
	trackExists, err := store.trackExists(field, value)
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
func (store Datastore) getTrackDataByField(field string, value interface{}) (track Track, err error) {
	track = Track{}
	err = store.DB.Get(&track, "SELECT id, url, fingerprint, duration FROM track WHERE "+field+"=$1", value)
	if err != nil {
		if (err.Error() == "sql: no rows in result set") {
			err = errors.New("Track Not Found")
		}
		return
	}
	track.Tags, err = store.getAllTagsForTrack(track.ID)
	return
}

/**
 * Checks whether a track exists based on a given field
 *
 */
func (store Datastore) trackExists(field string, value interface{}) (found bool, err error) {
	err = store.DB.Get(&found, "SELECT 1 FROM track WHERE "+field+"=$1", value)
	if (err != nil && err.Error() == "sql: no rows in result set") {
		err = nil
	}
	return
}

/**
 * Sets the weighting for a given track
 *
 */
func (store Datastore) setTrackWeighting(trackid int, weighting float64) (err error) {
	oldWeighting, err := store.getTrackWeighting(trackid)
	if err != nil { return }
	diff := weighting - oldWeighting
	max, err := store.getMaxCumWeighting()
	if (err != nil) { return }
	_, err = store.DB.Exec("UPDATE track SET cum_weighting = cum_weighting - $1 WHERE cum_weighting > (SELECT cum_weighting FROM track WHERE id = $2)", oldWeighting, trackid)
	if (err != nil) { return }
	_, err = store.DB.Exec("UPDATE track SET weighting = $1, cum_weighting = $2 WHERE id = $3", weighting, max+diff, trackid)
	return
}

/**
 * Gets the weighting of a given track
 *
 */
func (store Datastore) getTrackWeighting(trackid int) (weighting float64, err error) {
    err = store.DB.Get(&weighting, "SELECT weighting FROM track WHERE id=$1", trackid)
	if (err != nil && err.Error() == "sql: no rows in result set") {
		err = errors.New("Track Not Found")
	}
    return
}
/**
 * Gets the highest cumulative weighting value (defaults to 0)
 *
 */
func (store Datastore) getMaxCumWeighting() (maxcumweighting float64, err error) {
	err = store.DB.Get(&maxcumweighting, "SELECT IFNULL(MAX(cum_weighting), 0) FROM track")
	if (maxcumweighting < 0) { err = errors.New("cum_weightings are negative, max: "+strconv.FormatFloat(maxcumweighting, 'f', -1, 64)) }
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
 * Gets data about a random set of tracks from the database
 *
 */
func (store Datastore) getRandomTracks(count int) (tracks []Track, err error) {
	max, err := store.getMaxCumWeighting()
	if (err != nil) { return }

	tracks = []Track{}
	if (max == 0) { return }

	for i := 0; i < count; i++ {
		track := Track{}
		weighting := rand.Float64() * max
	    err = store.DB.Get(&track, "SELECT id, url, fingerprint, duration FROM track WHERE cum_weighting > $1 ORDER BY cum_weighting ASC LIMIT 1", weighting)
	    if err != nil { return }
		track.Tags, err = store.getAllTagsForTrack(track.ID)
		if err != nil { return }
		tracks = append(tracks, track)
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
	writeJSONResponse(w, tracks, err)
}

/**
 * Writes a http response with a JSON representation of a given track
 */
func writeTrackDataByField(store Datastore, w http.ResponseWriter, field string, value interface{}) {
	track, err := store.getTrackDataByField(field, value)
	writeJSONResponse(w, track, err)
}

/**
 * Writes a http response with the weighting of a given track
 */
func writeWeighting(store Datastore, w http.ResponseWriter, trackid int) {
	weighting, err := store.getTrackWeighting(trackid)
	weightingval := strconv.FormatFloat(weighting, 'f', -1, 64)
	writePlainResponse(w, weightingval, err)
}

/**
 * Write a http response with a JSON representation of a random 20 tracks
 */
func writeRandomTracks(store Datastore, w http.ResponseWriter) {
	tracks, err := store.getRandomTracks(20)
	writeJSONResponse(w, tracks, err)
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
		if err == nil {
			filterfield = "id"
			filtervalue = pathparts[1]
			trackid = id
		}
	} else if (len(trackurl) > 0) {
		filterfield = "url"
		filtervalue = trackurl
	} else if (len(fingerprint) > 0) {
		filterfield = "fingerprint"
		filtervalue = fingerprint
	}
	if (filterfield == "") {
		if (len(pathparts) <= 1) {
			if (r.Method == "GET") {
				writeAllTrackData(store, w)
			} else {
				MethodNotAllowed(w, []string{"GET"})
			}
		} else {
			switch pathparts[1] {
				case "random":
					writeRandomTracks(store, w)
				default:
					http.Error(w, "Track Endpoint Not Found", http.StatusNotFound)
			}
		}
	} else if (len(pathparts) > 2) {
		switch pathparts[2] {
			case "weighting":
				switch r.Method {
					case "PUT":
						body, err := ioutil.ReadAll(r.Body)
						if err != nil {
							http.Error(w, err.Error(), http.StatusInternalServerError)
							return
						}
						weighting, err := strconv.ParseFloat(string(body), 64)
						if err != nil {
							http.Error(w, "Weighting must be a number", http.StatusBadRequest)
							return
						}
						err = store.setTrackWeighting(trackid, weighting)
						if err != nil {
							http.Error(w, err.Error(), http.StatusInternalServerError)
							return
						}
						fallthrough
					case "GET":
						writeWeighting(store, w, trackid)
					default:
						MethodNotAllowed(w, []string{"GET", "PUT"})
				}
			default:
				http.Error(w, "Track Endpoint Not Found", http.StatusNotFound)
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
