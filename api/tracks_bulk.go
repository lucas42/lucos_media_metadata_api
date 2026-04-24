package main

import (
	"errors"
	"net/http"
	"strconv"
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
 * Updates a set of tracks based on get parameters
 */
func updateMultipleTracks(store Datastore, r *http.Request, updatesForTracks Track) (result SearchResult, action string, err error) {
	tracks, totalPages, _, _, err := queryMultipleTracksV3(store, r)
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
	return
}
