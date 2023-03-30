package main

import (
	"strconv"
)

/**
 * A struct for holding data about a given track
 */
type SearchResult struct {
	Tracks []Track `json:"tracks"`
	TotalPages int `json:"totalPages"`
}

/**
 * Searches for tracks with a tag value containing the given query
 *
 */
func (store Datastore) trackSearch(query string, offset int, limit int) (tracks []Track, totalTracks int, err error) {
	tracks = []Track{}
	query = "%"+query+"%"
	err = store.DB.Select(&tracks, "SELECT id, url, fingerprint, duration, weighting FROM tag LEFT JOIN track ON tag.trackid = track.id WHERE value LIKE $1 UNION SELECT id, url, fingerprint, duration, weighting FROM track WHERE url LIKE $1 GROUP BY id ORDER BY id LIMIT $2, $3", query, offset, limit)
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
	err = store.DB.Get(&totalTracks, "SELECT COUNT(*) FROM (SELECT id, url, fingerprint, duration, weighting FROM tag LEFT JOIN track ON tag.trackid = track.id WHERE value LIKE $1 UNION SELECT id, url, fingerprint, duration, weighting FROM track WHERE url LIKE $1 GROUP BY id ORDER BY id)", query)
	return
}

/**
 * Searches for tracks based on a map of predicates and their values
 *
 */
func (store Datastore) searchByPredicates(predicates map[string]string, offset int, limit int) (tracks []Track, totalTracks int, err error) {
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
	countQuery := dbQuery
	dbQuery += " ORDER BY id LIMIT ?, ?"
	values = append(values, offset, limit)

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
	err = store.DB.Get(&totalTracks, "SELECT COUNT(*) FROM ("+countQuery+")", values...)
	return
}
