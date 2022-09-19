package main

import (
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"math"
)


/**
 * Runs a basic search and write the output to the http response
 */
func getMultipleTracks(store Datastore, w http.ResponseWriter, r *http.Request) {
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
	page, err := strconv.Atoi(r.URL.Query().Get("page"))

	// If there's any doubt about page number, start at page 1
	if err != nil {
		page = 1
	}
	limit := 20
	startid := limit * (page - 1)
	var result SearchResult
	var totalTracks int
	if (query != "") {
		result.Tracks, totalTracks, err = store.trackSearch(query, limit, startid)
	} else {
		result.Tracks, totalTracks, err = store.searchByPredicates(predicates, limit, startid)
	}

	result.TotalPages = int(math.Ceil(float64(totalTracks) / float64(limit)))
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
			if r.Method == "GET" {
				getMultipleTracks(store, w, r)
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
	} else if len(pathparts) > 2 {
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
				statusCode := http.StatusInternalServerError
				if strings.HasPrefix(err.Error(), "Duplicate:") {
					statusCode = http.StatusBadRequest
				}
				http.Error(w, err.Error(), statusCode)
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
