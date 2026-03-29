package main

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
)

// TrackV3 is the v3 wire representation of a track.
// Tags use the structured format: all predicates map to arrays of {name, uri} objects.
// Uses "id" instead of "trackid" per ADR §7.
// Excludes debug weighting fields per ADR §7.
type TrackV3 struct {
	Fingerprint string        `json:"fingerprint"`
	Duration    int           `json:"duration"`
	URL         string        `json:"url"`
	ID          int           `json:"id"`
	Tags        TagListV3     `json:"tags"`
	Weighting   float64       `json:"weighting"`
	Collections *[]Collection `json:"collections,omitempty"`
}

// SearchResultV3 includes richer pagination per ADR §7.
type SearchResultV3 struct {
	Tracks      []TrackV3 `json:"tracks"`
	TotalPages  int       `json:"totalPages"`
	Page        int       `json:"page"`
	TotalTracks int       `json:"totalTracks"`
}

// V3Error is the structured JSON error response per ADR §7.
type V3Error struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// writeV3ErrorResponse writes a structured JSON error response.
func writeV3ErrorResponse(w http.ResponseWriter, statusCode int, message string, code string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, max-age=0, no-store, must-revalidate")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(V3Error{Error: message, Code: code})
}

// writeV3Error maps common errors to structured JSON responses.
func writeV3Error(w http.ResponseWriter, err error) {
	msg := err.Error()
	if strings.HasSuffix(msg, " Not Found") {
		writeV3ErrorResponse(w, http.StatusNotFound, msg, "not_found")
	} else if strings.HasPrefix(msg, "Duplicate:") {
		writeV3ErrorResponse(w, http.StatusBadRequest, msg, "duplicate")
	} else if strings.HasSuffix(msg, "not allowed") {
		writeV3ErrorResponse(w, http.StatusBadRequest, msg, "bad_request")
	} else {
		writeV3ErrorResponse(w, http.StatusInternalServerError, msg, "internal_error")
		slog.Error("Internal Server Error", slog.Any("error", err))
	}
}

// TrackToV3 converts an internal Track to a TrackV3 for v3 serialisation.
func TrackToV3(t Track) TrackV3 {
	return TrackV3{
		Fingerprint: t.Fingerprint,
		Duration:    t.Duration,
		URL:         t.URL,
		ID:          t.ID,
		Tags:        TagListToV3(t.Tags),
		Weighting:   t.Weighting,
		Collections: t.Collections,
	}
}

// DecodeTrackV3 decodes a JSON request body into a TrackV3.
func DecodeTrackV3(r io.Reader) (TrackV3, error) {
	track := new(TrackV3)
	err := json.NewDecoder(r).Decode(track)
	return *track, err
}

// updateTagsV3 updates tags for a track using the v3 multi-value semantics.
// For each predicate, all existing values are replaced with the provided array.
// Empty arrays delete the predicate's tags.
// Stores both name (value) and uri per tag row.
func (store Datastore) updateTagsV3(trackid int, tags TagListV3) (err error) {
	// Group tags by predicate
	type tagEntry struct {
		Value string
		URI   string
	}
	byPredicate := make(map[string][]tagEntry)
	for _, tag := range tags {
		byPredicate[tag.PredicateID] = append(byPredicate[tag.PredicateID], tagEntry{Value: tag.Value, URI: tag.URI})
	}
	for predicate, entries := range byPredicate {
		// Filter out empty values
		nonEmpty := make([]tagEntry, 0, len(entries))
		for _, e := range entries {
			if e.Value != "" {
				nonEmpty = append(nonEmpty, e)
			}
		}
		if len(nonEmpty) == 0 {
			err = store.deleteTag(trackid, predicate)
			if err != nil {
				return
			}
			continue
		}
		// Ensure predicate exists
		hasPredicate, err2 := store.hasPredicate(predicate)
		if err2 != nil {
			return err2
		}
		if !hasPredicate {
			err = store.createPredicate(predicate)
			if err != nil {
				return
			}
		}
		// Ensure track exists
		trackFound, err2 := store.trackExists("id", trackid)
		if err2 != nil {
			return err2
		}
		if !trackFound {
			return errors.New("Unknown Track")
		}
		// Replace all values for this predicate in a transaction
		tx, err2 := store.DB.Beginx()
		if err2 != nil {
			return err2
		}
		_, err = tx.Exec("DELETE FROM tag WHERE trackid = $1 AND predicateid = $2", trackid, predicate)
		if err != nil {
			_ = tx.Rollback()
			return
		}
		for _, e := range nonEmpty {
			_, err = tx.Exec("INSERT INTO tag(trackid, predicateid, value, uri) VALUES($1, $2, $3, $4)", trackid, predicate, e.Value, e.URI)
			if err != nil {
				_ = tx.Rollback()
				return
			}
		}
		err = tx.Commit()
		if err != nil {
			return
		}
	}
	return
}

// updateTagsV3IfMissing updates tags only if the predicate has no existing values.
// Stores both name (value) and uri per tag row.
func (store Datastore) updateTagsV3IfMissing(trackid int, tags TagListV3) (err error) {
	type tagEntry struct {
		Value string
		URI   string
	}
	byPredicate := make(map[string][]tagEntry)
	for _, tag := range tags {
		byPredicate[tag.PredicateID] = append(byPredicate[tag.PredicateID], tagEntry{Value: tag.Value, URI: tag.URI})
	}
	for predicate, entries := range byPredicate {
		_, err = store.getTagValue(trackid, predicate)
		if err != nil && err.Error() == "Tag Not Found" {
			err = nil
			// Filter empty values
			nonEmpty := make([]tagEntry, 0, len(entries))
			for _, e := range entries {
				if e.Value != "" {
					nonEmpty = append(nonEmpty, e)
				}
			}
			if len(nonEmpty) == 0 {
				continue
			}
			// Ensure predicate exists
			hasPredicate, err2 := store.hasPredicate(predicate)
			if err2 != nil {
				return err2
			}
			if !hasPredicate {
				err = store.createPredicate(predicate)
				if err != nil {
					return
				}
			}
			// Insert all values in a single transaction
			tx, err2 := store.DB.Beginx()
			if err2 != nil {
				return err2
			}
			for _, e := range nonEmpty {
				_, err = tx.Exec("INSERT INTO tag(trackid, predicateid, value, uri) VALUES($1, $2, $3, $4)", trackid, predicate, e.Value, e.URI)
				if err != nil {
					_ = tx.Rollback()
					return
				}
			}
			err = tx.Commit()
			if err != nil {
				return
			}
		} else if err != nil {
			return
		}
	}
	return
}

// getTrackDataByFieldV3 gets a track and returns it in v3 format.
func (store Datastore) getTrackDataByFieldV3(field string, value interface{}) (track TrackV3, err error) {
	internalTrack, err := store.getTrackDataByField(field, value)
	if err != nil {
		return
	}
	track = TrackToV3(internalTrack)
	return
}

// TracksV3Controller handles all requests to /v3/tracks endpoints.
func (store Datastore) TracksV3Controller(w http.ResponseWriter, r *http.Request) {
	trackurl := r.URL.Query().Get("url")
	fingerprint := r.URL.Query().Get("fingerprint")
	normalisedpath := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v3/tracks"), "/")
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
	slog.Debug("Tracks v3 controller", "method", r.Method, "filterfield", filterfield, "filtervalue", filtervalue, "pathparts", pathparts)
	if filterfield == "" {
		if len(pathparts) <= 1 {
			switch r.Method {
			case "PATCH":
				store.patchMultipleTracksV3(w, r)
			case "GET":
				store.getMultipleTracksV3(w, r)
			default:
				MethodNotAllowed(w, []string{"GET", "PATCH"})
			}
		} else {
			switch pathparts[1] {
			case "random":
				store.writeRandomTracksV3(w)
			default:
				writeV3ErrorResponse(w, http.StatusNotFound, "Track Endpoint Not Found", "not_found")
			}
		}
	} else if len(pathparts) > 2 {
		switch pathparts[2] {
		case "weighting":
			switch r.Method {
			case "PUT":
				body, err := io.ReadAll(r.Body)
				if err != nil {
					writeV3Error(w, err)
					return
				}
				weighting, err := strconv.ParseFloat(string(body), 64)
				if err != nil {
					writeV3ErrorResponse(w, http.StatusBadRequest, "Weighting must be a number", "bad_request")
					return
				}
				err = store.setTrackWeighting(trackid, weighting)
				if err != nil {
					writeV3Error(w, err)
					return
				}
				fallthrough
			case "GET":
				writeWeighting(store, w, trackid)
			default:
				MethodNotAllowed(w, []string{"GET", "PUT"})
			}
		default:
			writeV3ErrorResponse(w, http.StatusNotFound, "Track Endpoint Not Found", "not_found")
		}
	} else {
		switch r.Method {
		case "PATCH":
			fallthrough
		case "PUT":
			store.putPatchSingleTrackV3(w, r, filterfield, filtervalue, trackid, trackurl, fingerprint)
		case "GET":
			store.getSingleTrackV3(w, filterfield, filtervalue)
		case "DELETE":
			deleteTrackHandler(store, w, trackid)
		default:
			MethodNotAllowed(w, []string{"GET", "PUT", "PATCH", "DELETE"})
		}
	}
}

func (store Datastore) getSingleTrackV3(w http.ResponseWriter, filterfield string, filtervalue interface{}) {
	track, err := store.getTrackDataByFieldV3(filterfield, filtervalue)
	if err != nil {
		writeV3Error(w, err)
		return
	}
	writeJSONResponse(w, track, nil)
}

func (store Datastore) getMultipleTracksV3(w http.ResponseWriter, r *http.Request) {
	tracks, totalPages, totalTracks, page, err := queryMultipleTracksV3(store, r)
	if err != nil {
		writeV3Error(w, err)
		return
	}
	var result SearchResultV3
	result.Tracks = make([]TrackV3, len(tracks))
	for i, t := range tracks {
		result.Tracks[i] = TrackToV3(t)
	}
	result.TotalPages = totalPages
	result.TotalTracks = totalTracks
	result.Page = page
	writeJSONResponse(w, result, nil)
}

func (store Datastore) writeRandomTracksV3(w http.ResponseWriter) {
	tracks, err := store.getRandomTracks(20)
	if err != nil {
		writeV3Error(w, err)
		return
	}
	var result SearchResultV3
	result.Tracks = make([]TrackV3, len(tracks))
	for i, t := range tracks {
		result.Tracks[i] = TrackToV3(t)
	}
	result.TotalTracks = len(tracks)
	result.Page = 1
	if len(tracks) > 0 {
		result.TotalPages = 1
	}
	writeJSONResponse(w, result, nil)
}

func (store Datastore) patchMultipleTracksV3(w http.ResponseWriter, r *http.Request) {
	trackV3, err := DecodeTrackV3(r.Body)
	if err != nil {
		writeV3ErrorResponse(w, http.StatusBadRequest, err.Error(), "bad_request")
		return
	}
	if trackV3.ID != 0 {
		writeV3ErrorResponse(w, http.StatusBadRequest, "Can't bulk update id", "bad_request")
		return
	}
	if trackV3.URL != "" {
		writeV3ErrorResponse(w, http.StatusBadRequest, "Can't bulk update url", "bad_request")
		return
	}
	if trackV3.Fingerprint != "" {
		writeV3ErrorResponse(w, http.StatusBadRequest, "Can't bulk update fingerprint", "bad_request")
		return
	}
	// Strip tags from internal track — handle them via the v3 path per track
	v3Tags := trackV3.Tags
	internalTrack := trackV3ToInternal(trackV3)
	internalTrack.Tags = nil
	onlyMissing := (r.Header.Get("If-None-Match") == "*")
	_, action, err := updateMultipleTracks(store, r, internalTrack)
	if err != nil {
		writeV3Error(w, err)
		return
	}
	// Apply v3 tags to each matched track
	if v3Tags != nil {
		tracks, _, _, _, err2 := queryMultipleTracksV3(store, r)
		if err2 != nil {
			writeV3Error(w, err2)
			return
		}
		for _, t := range tracks {
			if onlyMissing {
				err = store.updateTagsV3IfMissing(t.ID, v3Tags)
			} else {
				err = store.updateTagsV3(t.ID, v3Tags)
			}
			if err != nil {
				writeV3Error(w, err)
				return
			}
		}
	}
	// Re-query to get v3-formatted results
	tracks, totalPages, totalTracks, page, err := queryMultipleTracksV3(store, r)
	if err != nil {
		writeV3Error(w, err)
		return
	}
	var resultV3 SearchResultV3
	resultV3.TotalPages = totalPages
	resultV3.TotalTracks = totalTracks
	resultV3.Page = page
	resultV3.Tracks = make([]TrackV3, len(tracks))
	for i, t := range tracks {
		resultV3.Tracks[i] = TrackToV3(t)
	}
	w.Header().Set("Track-Action", action)
	writeJSONResponse(w, resultV3, nil)
}

func (store Datastore) putPatchSingleTrackV3(w http.ResponseWriter, r *http.Request, filterfield string, filtervalue interface{}, trackid int, trackurl string, fingerprint string) {
	trackV3, err := DecodeTrackV3(r.Body)
	if err != nil {
		writeV3ErrorResponse(w, http.StatusBadRequest, err.Error(), "bad_request")
		return
	}
	switch filterfield {
	case "id":
		trackV3.ID = trackid
	case "url":
		trackV3.URL = trackurl
	case "fingerprint":
		trackV3.Fingerprint = fingerprint
	}
	onlyMissing := (r.Header.Get("If-None-Match") == "*")
	existingTrack, err := store.getTrackDataByField(filterfield, filtervalue)

	// Convert to internal Track for the updateCreateTrackDataByField call.
	// Tags are included so updateNeeded can detect tag changes and Loganne fires.
	// After the call, we overwrite tags via the v3-aware path.
	v3Tags := trackV3.Tags
	internalTrack := trackV3ToInternal(trackV3)

	if r.Method == "PATCH" {
		if err != nil && err.Error() == "Track Not Found" {
			writeV3ErrorResponse(w, http.StatusNotFound, "Track Not Found", "not_found")
			return
		}
		_, _, err = store.updateCreateTrackDataByField(filterfield, filtervalue, internalTrack, existingTrack, onlyMissing)
	} else {
		missingFields := []string{}
		if internalTrack.Fingerprint == "" {
			missingFields = append(missingFields, "fingerprint")
		}
		if internalTrack.URL == "" {
			missingFields = append(missingFields, "url")
		}
		if internalTrack.Duration == 0 {
			missingFields = append(missingFields, "duration")
		}
		if len(missingFields) > 0 {
			writeV3ErrorResponse(w, http.StatusBadRequest, "Missing fields \""+strings.Join(missingFields, "\" and \"")+"\"", "bad_request")
			return
		}
		_, _, err = store.updateCreateTrackDataByField(filterfield, filtervalue, internalTrack, existingTrack, onlyMissing)
	}
	if err != nil {
		writeV3Error(w, err)
		return
	}

	// Overwrite tags via the v3-aware path which handles multi-value predicates and URIs.
	if v3Tags != nil {
		storedTrack, err := store.getTrackDataByField(filterfield, filtervalue)
		if err != nil {
			writeV3Error(w, err)
			return
		}
		if onlyMissing {
			err = store.updateTagsV3IfMissing(storedTrack.ID, v3Tags)
		} else {
			err = store.updateTagsV3(storedTrack.ID, v3Tags)
		}
		if err != nil {
			writeV3Error(w, err)
			return
		}
	}

	// Re-fetch in v3 format after the update
	savedTrack, err := store.getTrackDataByFieldV3(filterfield, filtervalue)
	if err != nil {
		writeV3Error(w, err)
		return
	}
	writeJSONResponse(w, savedTrack, nil)
}

// trackV3ToInternal converts a TrackV3 to the internal Track type.
func trackV3ToInternal(v3 TrackV3) Track {
	return Track{
		Fingerprint: v3.Fingerprint,
		Duration:    v3.Duration,
		URL:         v3.URL,
		ID:          v3.ID,
		Tags:        v3.Tags.ToTagList(),
		Weighting:   v3.Weighting,
		Collections: v3.Collections,
	}
}

// queryMultipleTracksV3 wraps queryMultipleTracks with richer pagination data.
func queryMultipleTracksV3(store Datastore, r *http.Request) (tracks []Track, totalPages int, totalTracks int, page int, err error) {
	standardLimit := 20
	rawPage := r.URL.Query().Get("page")
	page, parseErr := strconv.Atoi(rawPage)
	if parseErr != nil || page < 1 {
		page = 1
	}
	if rawPage == "all" {
		page = 1
	}

	var query string
	predicates := make(map[string]string)
	for key, value := range r.URL.Query() {
		if key == "q" {
			query = value[0]
		}
		if strings.HasPrefix(key, "p.") {
			predicates[key[2:]] = value[0]
		}
	}
	offset, limit := parsePageParam(rawPage, standardLimit)
	if query != "" {
		tracks, totalTracks, err = store.trackSearch(query, offset, limit)
	} else {
		tracks, totalTracks, err = store.searchByPredicates(predicates, offset, limit)
	}
	totalPages = int(math.Ceil(float64(totalTracks) / float64(standardLimit)))
	return
}
