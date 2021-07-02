package main

import (
	"net/http"
	"strconv"
	"strings"
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
func (store Datastore) trackSearch(query string, page int) (tracks []Track, err error) {
	limit := 20
	startid := limit * (page - 1)
	tracks = []Track{}
	query = "%"+query+"%"
	err = store.DB.Select(&tracks, "SELECT id, url, fingerprint, duration, weighting FROM tag LEFT JOIN track ON tag.trackid = track.id WHERE value LIKE $1 UNION SELECT id, url, fingerprint, duration, weighting FROM track WHERE url LIKE $1 GROUP BY id ORDER BY id LIMIT $2, $3", query, startid, limit)
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
 * Searches for tracks based on a map of predicates and their values
 *
 */
func (store Datastore) searchByPredicates(predicates map[string]string, page int) (tracks []Track, err error) {
	limit := 20
	startid := limit * (page - 1)
	tracks = []Track{}
	dbQuery := "SELECT id, url, fingerprint, duration, weighting FROM track"
	var values []interface{}
	tagCount := 0
	for key, value := range predicates {
		tagCount++
		table := "tag"+strconv.Itoa(tagCount)
		dbQuery += " INNER JOIN tag AS "+table+" ON "+table+".trackid = track.id AND "+table+".predicateid = ? AND "+table+".value = ?"
		values = append(values, key, value)
	}
	dbQuery += " ORDER BY id LIMIT ?, ?"
	values = append(values, startid, limit)

	err = store.DB.Select(&tracks, dbQuery, values...)
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
func basicSearch(store Datastore, w http.ResponseWriter, r *http.Request) {
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
	if query == "" && len(predicates) == 0 {
		http.Error(w, "No query given", http.StatusBadRequest)
		return
	}
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil {
		page = 1
	} // If there's any doubt about page number, start at page 1
	var result SearchResult
	if (query != "") {
		result.Tracks, err = store.trackSearch(query, page)
	} else {
		result.Tracks, err = store.searchByPredicates(predicates, page)
	}
	writeJSONResponse(w, result, err)
}

/**
 * A controller for handling search requests
 */
func (store Datastore) SearchController(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
		case "GET":
			basicSearch(store, w, r)
		default:
			MethodNotAllowed(w, []string{"GET"})
	}
}