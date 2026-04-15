package main

import (
	"strconv"
	"strings"
)

/**
 * A struct for holding data about a given track
 */
type SearchResult struct {
	Tracks []Track `json:"tracks"`
	TotalPages int `json:"totalPages"`
}

/**
 * Searches for tracks based on a map of predicates and their values, and/or a map of predicates and their URIs.
 *
 */
func (store Datastore) searchByPredicates(predicates map[string]string, uriPredicates map[string]string, offset int, limit int) (tracks []Track, totalTracks int, err error) {
	tracks = []Track{}
	dbQuery := "SELECT id, url, fingerprint, duration, weighting FROM track"
	var values []interface{}
	var whereClauses []string
	tagCount := 0
	for key, value := range predicates {
		tagCount++
		table := "tag"+strconv.Itoa(tagCount)
		if value == "" {
			// Empty value means search for tracks missing this predicate
			dbQuery += " LEFT JOIN tag AS "+table+" ON "+table+".trackid = track.id AND "+table+".predicateid = ?"
			values = append(values, key)
			whereClauses = append(whereClauses, table+".value IS NULL")
		} else {
			dbQuery += " INNER JOIN tag AS "+table+" ON "+table+".trackid = track.id AND "+table+".predicateid = ? AND "+table+".value = ?"
			values = append(values, key, value)
		}
	}
	for key, uri := range uriPredicates {
		tagCount++
		table := "tag"+strconv.Itoa(tagCount)
		dbQuery += " INNER JOIN tag AS "+table+" ON "+table+".trackid = track.id AND "+table+".predicateid = ? AND "+table+".uri = ?"
		values = append(values, key, uri)
	}
	if len(whereClauses) > 0 {
		dbQuery += " WHERE " + strings.Join(whereClauses, " AND ")
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
		collections, err := store.getCollectionsByTrack(track.ID)
		if err != nil {
			return tracks, totalTracks, err
		}
		track.Collections = &collections
	}
	err = store.DB.Get(&totalTracks, "SELECT COUNT(*) FROM ("+countQuery+")", values...)
	return
}
