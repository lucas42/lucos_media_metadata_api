package main

import (
	"net/http"
)

/**
 * A struct for holding data about a given track
 */
type SearchResult struct {
	Tracks []Track `json:"tracks"`
}

/**
 * Searches for tracks with a tag value containing the given query
 *
 */
func (store Datastore) trackSearch(query string) (tracks []Track, err error) {
	limit := 20
	startid := 0 // TODO: add pagination
	tracks = []Track{}
	query = "%"+query+"%"
	err = store.DB.Select(&tracks, "SELECT id, url, fingerprint, duration, weighting FROM tag LEFT JOIN track ON tag.trackid = track.id WHERE value LIKE $1 GROUP BY trackid ORDER BY trackid LIMIT 0, 20", query, startid, limit)
	if err != nil {
		return
	}

	// Loop through all the tracks and add tags for each one
	for i := range tracks {
		track := &tracks[i]
		track.Tags, err = store.getAllTagsForTrack(track.ID)
		if err != nil {
			return
		}
	}
	return
}


/**
 * Runs a basic search and write the output to the http response
 */
func basicSearch(store Datastore, w http.ResponseWriter, query string) {
	var result SearchResult
	var err error
	result.Tracks, err = store.trackSearch(query)
	writeJSONResponse(w, result, err)
}

/**
 * A controller for handling search requests
 */
func (store Datastore) SearchController(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "No query given", http.StatusBadRequest)
		return
	}
	switch r.Method {
		case "GET":
			basicSearch(store, w, query)
		default:
			MethodNotAllowed(w, []string{"GET"})
	}
}