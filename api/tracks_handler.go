package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/jmoiron/sqlx"
)

// TrackV3 is the v3 wire representation of a track.
// Tags use the structured format: each predicate maps to an array of {name, uri} objects.
// Uses "id" instead of "trackid" per ADR §7.
// Excludes debug weighting fields per ADR §7.
// A nil Tags map means "tags not provided" (e.g. absent from a PATCH body).
type TrackV3 struct {
	Fingerprint string                       `json:"fingerprint"`
	Duration    int                          `json:"duration"`
	URL         string                       `json:"url"`
	ID          int                          `json:"id"`
	Tags        map[string][]TagValueV3      `json:"tags"`
	Weighting   float64                      `json:"weighting"`
	Collections *[]Collection               `json:"collections,omitempty"`
}

// SearchResultV3 includes richer pagination per ADR §7.
type SearchResultV3 struct {
	Tracks      []TrackV3 `json:"tracks"`
	TotalPages  int       `json:"totalPages"`
	Page        int       `json:"page"`
	TotalTracks int       `json:"totalTracks"`
}

// V3Error is the structured JSON error response per ADR §7.
// Predicate is an optional extension for tag-level errors identifying the offending predicate.
type V3Error struct {
	Error     string `json:"error"`
	Code      string `json:"code"`
	Predicate string `json:"predicate,omitempty"`
}

// writeV3ErrorResponse writes a structured JSON error response.
func writeV3ErrorResponse(w http.ResponseWriter, statusCode int, message string, code string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, max-age=0, no-store, must-revalidate")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(V3Error{Error: message, Code: code})
}

// writeV3TagValidationError writes a 400 invalid_tag_value error for a specific predicate.
func writeV3TagValidationError(w http.ResponseWriter, predicate string, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, max-age=0, no-store, must-revalidate")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(V3Error{Error: message, Code: "invalid_tag_value", Predicate: predicate})
}

// validateTagsV3 validates tag values from a v3 write request.
// Returns the offending predicate name and an error message if any value is invalid:
//   - a nil slice for a predicate (use [] to clear, not null)
//   - a value with both name and uri empty (no identifying information)
func validateTagsV3(tags map[string][]TagValueV3) (predicate string, message string, invalid bool) {
	for pred, values := range tags {
		if values == nil {
			return pred, "tag value array must not be null; use [] to clear", true
		}
		for _, v := range values {
			if v.Name == "" && v.URI == "" {
				return pred, "tag value name must be non-empty", true
			}
		}
	}
	return "", "", false
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
	} else if strings.Contains(msg, "requires a URI") {
		writeV3ErrorResponse(w, http.StatusBadRequest, msg, "requires_uri")
	} else if strings.Contains(msg, "does not support URI-based filtering") {
		writeV3ErrorResponse(w, http.StatusBadRequest, msg, "bad_request")
	} else {
		writeV3ErrorResponse(w, http.StatusInternalServerError, msg, "internal_error")
		slog.Error("Internal Server Error", slog.Any("error", err))
	}
}


// TrackToV3 converts an internal Track to a TrackV3 for v3 serialisation.
func TrackToV3(t Track) TrackV3 {
	tags := make(map[string][]TagValueV3)
	for _, tag := range t.Tags {
		tags[tag.PredicateID] = append(tags[tag.PredicateID], TagValueV3{Name: tag.Value, URI: tag.URI})
	}
	return TrackV3{
		Fingerprint: t.Fingerprint,
		Duration:    t.Duration,
		URL:         t.URL,
		ID:          t.ID,
		Tags:        tags,
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
// For each predicate in the map, all existing values are replaced with the
// provided array. Empty arrays delete the predicate's tags.
// All predicate updates are applied atomically in a single transaction.
// Stores both name (value) and uri per tag row.
// Returns changed=true if any predicate's stored values actually differed from the desired values.
func (store Datastore) updateTagsV3(trackid int, tags map[string][]TagValueV3) (changed bool, err error) {
	type predicateUpdate struct {
		predicate string
		values    []TagValueV3 // empty slice means delete all values
	}

	// Filter empty values and validate constraints before touching the database.
	updates := make([]predicateUpdate, 0, len(tags))
	for predicate, values := range tags {
		nonEmpty := make([]TagValueV3, 0, len(values))
		for _, v := range values {
			// Include values with a name or a URI — a URI-only value is valid for
			// predicates that require a URI (e.g. album, where the client provides
			// only the URI and the API resolves the name).
			if v.Name != "" || v.URI != "" {
				nonEmpty = append(nonEmpty, v)
			}
		}
		if !IsMultiValue(predicate) && len(nonEmpty) > 1 {
			err = fmt.Errorf("multiple values for single-value predicate %q not allowed", predicate)
			return
		}
		config := GetPredicateConfig(predicate)
		// Resolve name to URI before RequiresURI validation so the resolved
		// URI satisfies that constraint.
		if config.ResolveNameToURI != nil {
			for i, v := range nonEmpty {
				if v.URI == "" && v.Name != "" {
					uri, resolveErr := config.ResolveNameToURI(store, v.Name)
					if resolveErr != nil {
						err = fmt.Errorf("could not resolve %q for predicate %q: %w", v.Name, predicate, resolveErr)
						return
					}
					nonEmpty[i].URI = uri
				}
			}
		}
		if config.RequiresURI {
			for _, v := range nonEmpty {
				if v.URI == "" {
					err = fmt.Errorf("predicate %q requires a URI", predicate)
					return
				}
			}
		}
		updates = append(updates, predicateUpdate{predicate, nonEmpty})
	}

	// Check track exists once.
	trackFound, err := store.trackExists("id", trackid)
	if err != nil {
		return
	}
	if !trackFound {
		err = errors.New("Unknown Track")
		return
	}

	// Ensure predicates exist for non-delete updates. createPredicate is
	// idempotent (REPLACE INTO) and safe outside the main transaction.
	for _, u := range updates {
		if len(u.values) == 0 {
			continue // delete path — predicate already exists
		}
		var hasPred bool
		if hasPred, err = store.hasPredicate(u.predicate); err != nil {
			return
		}
		if !hasPred {
			if err = store.createPredicate(u.predicate); err != nil {
				return
			}
		}
	}

	// Populate the Name field for URI-only tag values.
	for i, u := range updates {
		config := GetPredicateConfig(u.predicate)
		if config.ResolveURIToName != nil {
			for j, v := range u.values {
				if v.Name == "" && v.URI != "" {
					name, resolveErr := config.ResolveURIToName(store, v.URI)
					if resolveErr != nil {
						err = fmt.Errorf("tag URI %q for predicate %q does not match a known entity: %w", v.URI, u.predicate, resolveErr)
						return
					}
					updates[i].values[j].Name = name
				}
			}
		}
	}

	// Compare desired values against current DB state to detect actual changes.
	type tagRow struct {
		Value string `db:"value"`
		URI   string `db:"uri"`
	}
	for _, u := range updates {
		var current []tagRow
		if err = store.DB.Select(&current, "SELECT value, uri FROM tag WHERE trackid = $1 AND predicateid = $2", trackid, u.predicate); err != nil {
			return
		}
		if len(current) != len(u.values) {
			changed = true
			break
		}
		type tagKey = [2]string
		currentSet := make(map[tagKey]bool, len(current))
		for _, row := range current {
			currentSet[tagKey{row.Value, row.URI}] = true
		}
		for _, v := range u.values {
			if !currentSet[tagKey{v.Name, v.URI}] {
				changed = true
				break
			}
		}
		if changed {
			break
		}
	}
	if !changed {
		return
	}

	// Apply all tag writes atomically in a single transaction.
	var tx *sqlx.Tx
	if tx, err = store.DB.Beginx(); err != nil {
		return
	}
	for _, u := range updates {
		_, err = tx.Exec("DELETE FROM tag WHERE trackid = $1 AND predicateid = $2", trackid, u.predicate)
		if err != nil {
			_ = tx.Rollback()
			return
		}
		for _, v := range u.values {
			_, err = tx.Exec("INSERT INTO tag(trackid, predicateid, value, uri) VALUES($1, $2, $3, $4)", trackid, u.predicate, v.Name, v.URI)
			if err != nil {
				_ = tx.Rollback()
				return
			}
		}
	}
	err = tx.Commit()
	return
}

// updateTagsV3IfMissing updates tags only if the predicate has no existing values.
// All inserts are applied atomically in a single transaction.
// Stores both name (value) and uri per tag row.
// Returns changed=true if any tags were actually written.
func (store Datastore) updateTagsV3IfMissing(trackid int, tags map[string][]TagValueV3) (changed bool, err error) {
	type predicateInsert struct {
		predicate string
		values    []TagValueV3
	}

	// Determine which predicates are missing and have values to insert.
	inserts := make([]predicateInsert, 0, len(tags))
	for predicate, values := range tags {
		_, err = store.getTagValue(trackid, predicate)
		if err != nil && err.Error() == "Tag Not Found" {
			err = nil
			nonEmpty := make([]TagValueV3, 0, len(values))
			for _, v := range values {
				if v.Name != "" || v.URI != "" {
					nonEmpty = append(nonEmpty, v)
				}
			}
			if len(nonEmpty) == 0 {
				continue
			}
			config := GetPredicateConfig(predicate)
			// Resolve name to URI before RequiresURI validation. Only fires
			// here because the predicate is missing (tag will be written).
			if config.ResolveNameToURI != nil {
				for i, v := range nonEmpty {
					if v.URI == "" && v.Name != "" {
						uri, resolveErr := config.ResolveNameToURI(store, v.Name)
						if resolveErr != nil {
							err = fmt.Errorf("could not resolve %q for predicate %q: %w", v.Name, predicate, resolveErr)
							return
						}
						nonEmpty[i].URI = uri
					}
				}
			}
			// Populate Name from URI for URI-only values.
			if config.ResolveURIToName != nil {
				for i, v := range nonEmpty {
					if v.Name == "" && v.URI != "" {
						name, resolveErr := config.ResolveURIToName(store, v.URI)
						if resolveErr != nil {
							err = fmt.Errorf("tag URI %q for predicate %q does not match a known entity: %w", v.URI, predicate, resolveErr)
							return
						}
						nonEmpty[i].Name = name
					}
				}
			}
			if config.RequiresURI {
				for _, v := range nonEmpty {
					if v.URI == "" {
						err = fmt.Errorf("predicate %q requires a URI", predicate)
						return
					}
				}
			}
			// Ensure predicate exists. createPredicate is idempotent and safe
			// outside the main transaction.
			var hasPred bool
			if hasPred, err = store.hasPredicate(predicate); err != nil {
				return
			}
			if !hasPred {
				if err = store.createPredicate(predicate); err != nil {
					return
				}
			}
			inserts = append(inserts, predicateInsert{predicate, nonEmpty})
		} else if err != nil {
			return
		}
	}

	if len(inserts) == 0 {
		return
	}
	changed = true

	// Insert all missing predicate values atomically in a single transaction.
	var tx *sqlx.Tx
	if tx, err = store.DB.Beginx(); err != nil {
		return
	}
	for _, ins := range inserts {
		for _, v := range ins.values {
			_, err = tx.Exec("INSERT INTO tag(trackid, predicateid, value, uri) VALUES($1, $2, $3, $4)", trackid, ins.predicate, v.Name, v.URI)
			if err != nil {
				_ = tx.Rollback()
				return
			}
		}
	}
	err = tx.Commit()
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
			store.getSingleTrackV3(w, r, filterfield, filtervalue)
		case "DELETE":
			deleteTrackHandler(store, w, trackid)
		default:
			MethodNotAllowed(w, []string{"GET", "PUT", "PATCH", "DELETE"})
		}
	}
}

func (store Datastore) getSingleTrackV3(w http.ResponseWriter, r *http.Request, filterfield string, filtervalue interface{}) {
	isRDF, mime := prefersRDF(r)
	if isRDF {
		writeTrackRDFByField(store, w, filterfield, filtervalue, mime)
		return
	}
	track, err := store.getTrackDataByFieldV3(filterfield, filtervalue)
	if err != nil {
		writeV3Error(w, err)
		return
	}
	writeJSONResponse(w, track, nil)
}

func (store Datastore) getMultipleTracksV3(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("q") != "" {
		writeV3ErrorResponse(w, http.StatusBadRequest, "The q parameter is no longer supported. Use p. parameters for predicate-based search.", "bad_request")
		return
	}
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
	if r.URL.Query().Get("q") != "" {
		writeV3ErrorResponse(w, http.StatusBadRequest, "The q parameter is no longer supported. Use p. parameters for predicate-based search.", "bad_request")
		return
	}
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
	// Validate tag values before touching the database.
	if pred, msg, invalid := validateTagsV3(trackV3.Tags); invalid {
		writeV3TagValidationError(w, pred, msg)
		return
	}
	onlyMissing := (r.Header.Get("If-None-Match") == "*")

	// Query matched tracks once, used for both scalar and tag updates.
	matchedTracks, _, _, _, err := queryMultipleTracksV3(store, r)
	if err != nil {
		writeV3Error(w, err)
		return
	}

	// Update all matched tracks via updateCreateTrackDataByField.
	// Collect changed IDs to fire a single bulk Loganne event.
	changedTrackIDs := make(map[int]bool)
	for i := range matchedTracks {
		t := trackV3
		t.ID = matchedTracks[i].ID
		storedTrack, trackAction, trackErr := store.updateCreateTrackDataByField("id", matchedTracks[i].ID, t, matchedTracks[i], onlyMissing)
		if trackErr != nil {
			writeV3Error(w, trackErr)
			return
		}
		if trackAction != "noChange" {
			changedTrackIDs[storedTrack.ID] = true
		}
	}

	// Fire a single bulk Loganne event covering all per-track changes.
	action := "noChange"
	if len(changedTrackIDs) > 0 {
		action = "tracksUpdated"
		store.Loganne.post(action, strconv.Itoa(len(changedTrackIDs))+" tracks updated", Track{}, Track{})
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
	slog.Info("track update request", "method", r.Method, "filterField", filterfield, "filterValue", filtervalue, "onlyMissing", onlyMissing, "userAgent", r.Header.Get("User-Agent"))

	// Validate tag values before touching the database.
	if pred, msg, invalid := validateTagsV3(trackV3.Tags); invalid {
		writeV3TagValidationError(w, pred, msg)
		return
	}

	existingTrack, err := store.getTrackDataByField(filterfield, filtervalue)

	var action string
	if r.Method == "PATCH" {
		if err != nil && err.Error() == "Track Not Found" {
			writeV3ErrorResponse(w, http.StatusNotFound, "Track Not Found", "not_found")
			return
		}
		_, action, err = store.updateCreateTrackDataByField(filterfield, filtervalue, trackV3, existingTrack, onlyMissing)
	} else {
		missingFields := []string{}
		if trackV3.Fingerprint == "" {
			missingFields = append(missingFields, "fingerprint")
		}
		if trackV3.URL == "" {
			missingFields = append(missingFields, "url")
		}
		if trackV3.Duration == 0 {
			missingFields = append(missingFields, "duration")
		}
		if len(missingFields) > 0 {
			writeV3ErrorResponse(w, http.StatusBadRequest, "Missing fields \""+strings.Join(missingFields, "\" and \"")+"\"", "bad_request")
			return
		}
		_, action, err = store.updateCreateTrackDataByField(filterfield, filtervalue, trackV3, existingTrack, onlyMissing)
	}
	if err != nil {
		writeV3Error(w, err)
		return
	}

	// Re-fetch in v3 format after the update
	savedTrack, err := store.getTrackDataByFieldV3(filterfield, filtervalue)
	if err != nil {
		writeV3Error(w, err)
		return
	}
	w.Header().Set("Track-Action", action)
	writeJSONResponse(w, savedTrack, nil)
}

// trackV3ToInternal converts a TrackV3 to the internal Track type.
func trackV3ToInternal(v3 TrackV3) Track {
	var tags TagList
	for pred, values := range v3.Tags {
		for _, v := range values {
			tags = append(tags, Tag{PredicateID: pred, Value: v.Name, URI: v.URI})
		}
	}
	return Track{
		Fingerprint: v3.Fingerprint,
		Duration:    v3.Duration,
		URL:         v3.URL,
		ID:          v3.ID,
		Tags:        tags,
		Weighting:   v3.Weighting,
		Collections: v3.Collections,
	}
}

// queryMultipleTracksV3 parses predicate filters from request query parameters and returns matching tracks with pagination data.
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

	predicates := make(map[string]string)
	uriPredicates := make(map[string]string)
	for key, value := range r.URL.Query() {
		if strings.HasPrefix(key, "p.") {
			predicateName := key[2:]
			if strings.HasSuffix(predicateName, ".uri") {
				predicateName = predicateName[:len(predicateName)-4]
				if !GetPredicateConfig(predicateName).RequiresURI {
					err = fmt.Errorf("predicate %q does not support URI-based filtering", predicateName)
					return
				}
				uriPredicates[predicateName] = value[0]
			} else {
				predicates[predicateName] = value[0]
			}
		}
	}
	offset, limit := parsePageParam(rawPage, standardLimit)
	tracks, totalTracks, err = store.searchByPredicates(predicates, uriPredicates, offset, limit)
	totalPages = int(math.Ceil(float64(totalTracks) / float64(standardLimit)))
	return
}
