package main

import (
	"io/ioutil"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"math"
)

func parsePageParam(rawpage string, standardLimit int) (offset int, limit int) {
	if rawpage == "all" {
		return 0, -1
	}
	page, err := strconv.Atoi(rawpage)

	// If there's any doubt about page number, start at page 1
	if err != nil {
		page = 1
	}
	offset = standardLimit * (page - 1)
	return offset, standardLimit
}

/**
 * Run a basic search based on request GET parameters
 */
func queryMultipleTracks(store Datastore, r *http.Request) (tracks []Track, totalPages int, err error) {
	var query string
	predicates := make(map[string]string)
	for key, value := range r.URL.Query() {
		if key == "q" {
			query = value[0]
		}
		if strings.HasPrefix(key, "p.") {
			predicates[key[2:len(key)]] = value[0]
		}
	}
	standardLimit := 20
	offset, limit := parsePageParam(r.URL.Query().Get("page"), standardLimit)
	var totalTracks int
	if (query != "") {
		tracks, totalTracks, err = store.trackSearch(query, offset, limit)
	} else {
		tracks, totalTracks, err = store.searchByPredicates(predicates, offset, limit)
	}
	totalPages = int(math.Ceil(float64(totalTracks) / float64(standardLimit)))
	return tracks, totalPages, err
}

/**
 * Run a basic search and write the output to the http response
 */
func getMultipleTracks(store Datastore, w http.ResponseWriter, r *http.Request) {
	tracks, totalPages, err := queryMultipleTracks(store , r)
	var result SearchResult
	result.Tracks = tracks
	result.TotalPages = totalPages
	writeJSONResponse(w, result, err)
}

/**
 * Updates a set of tracks based on get parameters
 */
func updateMultipleTracks(store Datastore, r *http.Request, updatesForTracks Track) (result SearchResult, action string, err error) {
	tracks, totalPages, err := queryMultipleTracks(store, r)
	result.TotalPages = totalPages
	if err != nil {
		return
	}
	if err != nil {
		return
	}
	onlyMissing := (r.Header.Get("If-None-Match") == "*")

	result.Tracks = make([]Track, 0)
	action = "noChange"
	for i := range tracks {
		trackupdates := updatesForTracks
		trackupdates.ID = tracks[i].ID
		storedTrack, trackAction, trackError := store.updateCreateTrackDataByField("id", tracks[i].ID, trackupdates, tracks[i], onlyMissing)
		if trackError != nil {
			err = trackError
			return
		}
		if trackAction == "noChange" {
			continue
		}
		if trackAction != "trackUpdated" {
			err = errors.New("Unexpected action "+trackAction)
			return
		}
		result.Tracks = append(result.Tracks, storedTrack)
		action = "tracksUpdated"
	}
	if action != "noChange" {
		store.Loganne.post(action, strconv.Itoa(len(result.Tracks)) + " tracks updated", Track{}, Track{})
	}
	return
}
func updateMultipleTracksAndRespond(store Datastore, w http.ResponseWriter, r *http.Request, updatesForTracks Track) {
	result, action, err := updateMultipleTracks(store, r, updatesForTracks)
	w.Header().Set("Track-Action", action)
	writeJSONResponse(w, result, err)
}

/**
 * Write a http response with a JSON representation of a random 20 tracks
 */
func writeRandomTracksV2(store Datastore, w http.ResponseWriter) {
	tracks, err := store.getRandomTracks(20)
	var result SearchResult
	result.Tracks = tracks
	if (len(tracks) > 0) {
		result.TotalPages = 1
	}
	writeJSONResponse(w, result, err)
}

/**
 * A controller for handling all requests dealing with tracks
 */
func (store Datastore) TracksV2Controller(w http.ResponseWriter, r *http.Request) {
	trackurl := r.URL.Query().Get("url")
	fingerprint := r.URL.Query().Get("fingerprint")
	normalisedpath := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v2/tracks"), "/")
	pathparts := strings.Split(normalisedpath, "/")

	var trackid int
	var filtervalue interface{}
	var filterfield string
	if len(pathparts) > 1 {
		id, err := strconv.Atoi(pathparts[1])
		if err == nil {
			filterfield = "id"
			filtervalue = pathparts[1]
			trackid = id
		}
	} else if len(trackurl) > 0 {
		filterfield = "url"
		filtervalue = trackurl
	} else if len(fingerprint) > 0 {
		filterfield = "fingerprint"
		filtervalue = fingerprint
	}
	if filterfield == "" {
		if len(pathparts) <= 1 {
			switch r.Method {
			case "PATCH":
				track, err := DecodeTrack(r.Body)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				if track.ID != 0 {
					http.Error(w, "Can't bulk update id", http.StatusBadRequest)
					return
				}
				if track.URL != "" {
					http.Error(w, "Can't bulk update url", http.StatusBadRequest)
					return
				}
				if track.Fingerprint != "" {
					http.Error(w, "Can't bulk update fingerprint", http.StatusBadRequest)
					return
				}
				updateMultipleTracksAndRespond(store, w, r, track)
			case "GET":
				getMultipleTracks(store, w, r)
			default:
				MethodNotAllowed(w, []string{"GET", "PATCH"})
			}
		} else {
			switch pathparts[1] {
			case "random":
				writeRandomTracksV2(store, w)
			default:
				http.Error(w, "Track Endpoint Not Found", http.StatusNotFound)
			}
		}
	} else if len(pathparts) > 2 {
		switch pathparts[2] {
		case "weighting":
			switch r.Method {
			case "PUT":
				body, err := ioutil.ReadAll(r.Body)
				if err != nil {
					writeErrorResponse(w, err)
					return
				}
				weighting, err := strconv.ParseFloat(string(body), 64)
				if err != nil {
					http.Error(w, "Weighting must be a number", http.StatusBadRequest)
					return
				}
				err = store.setTrackWeighting(trackid, weighting)
				if err != nil {
					writeErrorResponse(w, err)
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
		case "PATCH":
			fallthrough
		case "PUT":
			var savedTrack Track
			var action string
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
			onlyMissing := (r.Header.Get("If-None-Match") == "*")
			existingTrack, err := store.getTrackDataByField(filterfield, filtervalue)
			if r.Method == "PATCH" {
				if err != nil && err.Error() == "Track Not Found" {
					http.Error(w, "Track Not Found", http.StatusNotFound)
					return
				}
				savedTrack, action, err = store.updateCreateTrackDataByField(filterfield, filtervalue, track, existingTrack, onlyMissing)
			} else {
				missingFields := []string{}
				if track.Fingerprint == "" {
					missingFields = append(missingFields, "fingerprint")
				}
				if track.URL == "" {
					missingFields = append(missingFields, "url")
				}
				if track.Duration == 0 {
					missingFields = append(missingFields, "duration")
				}
				if len(missingFields) > 0 {
					http.Error(w, "Missing fields \""+strings.Join(missingFields, "\" and \"")+"\"", http.StatusBadRequest)
					return
				}
				savedTrack, action, err = store.updateCreateTrackDataByField(filterfield, filtervalue, track, existingTrack, onlyMissing)
			}
			if err != nil {
				writeErrorResponse(w, err)
				return
			}
			w.Header().Set("Track-Action", action)
			writeJSONResponse(w, savedTrack, err)
		case "GET":
			writeTrackDataByField(store, w, filterfield, filtervalue)
		case "DELETE":
			deleteTrackHandler(store, w, trackid)
		default:
			MethodNotAllowed(w, []string{"GET", "PUT", "PATCH", "DELETE"})
		}
	}
}
