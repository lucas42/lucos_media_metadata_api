package main

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
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

